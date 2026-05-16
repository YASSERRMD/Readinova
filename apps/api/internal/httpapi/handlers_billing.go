package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"

	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	stripeSession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"

	"github.com/YASSERRMD/Readinova/apps/api/internal/billing"
)

// stripeInit sets the Stripe key exactly once at process startup (avoids data races).
var stripeInit sync.Once

func initStripe() {
	stripeInit.Do(func() {
		stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	})
}

// billingRoutes adds billing endpoints to the mux.
func (s *Server) billingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/billing/subscription", s.withAuth(s.handleGetSubscription))
	mux.HandleFunc("POST /v1/billing/checkout", s.withAuth(s.handleCreateCheckout))
	mux.HandleFunc("POST /v1/billing/portal", s.withAuth(s.handleBillingPortal))
	mux.HandleFunc("POST /v1/webhooks/stripe", s.handleStripeWebhook)
}

// GET /v1/billing/subscription
func (s *Server) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)

	var tier, status string
	if err := s.db.QueryRow(r.Context(),
		`SELECT tier, status FROM subscriptions WHERE organisation_id = $1`,
		claims.OrgID,
	).Scan(&tier, &status); err != nil {
		// No subscription row — treat as free.
		tier = string(billing.TierFree)
		status = "active"
	}

	limits := billing.LimitsFor(billing.Tier(tier))
	writeJSON(w, http.StatusOK, map[string]any{
		"tier":                  tier,
		"status":                status,
		"max_assessments":       limits.MaxAssessments,
		"max_team_members":      limits.MaxTeamMembers,
		"pdf_watermark":         limits.PDFWatermark,
		"evidence_connectors":   limits.EvidenceConnectors,
		"recommendation_engine": limits.RecommendationEngine,
		"audit_artefacts":       limits.AuditArtefacts,
	})
}

// POST /v1/billing/checkout
// Body: { "tier": "starter"|"growth"|"enterprise", "success_url": "...", "cancel_url": "..." }
func (s *Server) handleCreateCheckout(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims.Role != "owner" {
		writeError(w, http.StatusForbidden, "only owner may manage billing")
		return
	}

	var req struct {
		Tier       string `json:"tier"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Tier == "" {
		writeError(w, http.StatusBadRequest, "tier, success_url, cancel_url required")
		return
	}

	priceID := stripePriceID(req.Tier)
	if priceID == "" {
		writeError(w, http.StatusBadRequest, "unknown tier")
		return
	}

	initStripe()
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:          stripe.String(req.SuccessURL),
		CancelURL:           stripe.String(req.CancelURL),
		ClientReferenceID:   stripe.String(claims.OrgID),
		AllowPromotionCodes: stripe.Bool(true),
	}
	sess, err := stripeSession.New(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create checkout session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": sess.URL})
}

// POST /v1/billing/portal
func (s *Server) handleBillingPortal(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims.Role != "owner" {
		writeError(w, http.StatusForbidden, "only owner may manage billing")
		return
	}

	var req struct {
		ReturnURL string `json:"return_url"`
	}
	if err := decodeJSON(r, &req); err != nil || req.ReturnURL == "" {
		writeError(w, http.StatusBadRequest, "return_url required")
		return
	}

	var stripeCustomerID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT stripe_customer_id FROM subscriptions WHERE organisation_id = $1`,
		claims.OrgID,
	).Scan(&stripeCustomerID); err != nil || stripeCustomerID == "" {
		writeError(w, http.StatusNotFound, "no billing account found")
		return
	}

	initStripe()
	portalParams := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID),
		ReturnURL: stripe.String(req.ReturnURL),
	}
	portalSess, err := session.New(portalParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create billing portal session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": portalSess.URL})
}

// POST /v1/webhooks/stripe
func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), webhookSecret)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stripe signature")
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		s.handleCheckoutCompleted(r, event.Data.Raw)
	case "customer.subscription.updated",
		"customer.subscription.deleted":
		s.handleSubscriptionUpdate(r, event.Data.Raw)
	}

	w.WriteHeader(http.StatusOK)
}

// handleCheckoutCompleted processes a completed Stripe checkout session.
func (s *Server) handleCheckoutCompleted(r *http.Request, raw json.RawMessage) {
	var sess struct {
		ClientReferenceID string `json:"client_reference_id"`
		Customer          string `json:"customer"`
		Subscription      string `json:"subscription"`
		// price_id is resolved via subscription items in handleSubscriptionUpdate;
		// for checkout.session.completed we fall back to metadata if present.
		Metadata struct {
			Tier string `json:"tier"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &sess); err != nil || sess.ClientReferenceID == "" {
		return
	}

	// Determine tier: prefer metadata.tier set on the Checkout Session, otherwise default to starter.
	tier := billing.TierStarter
	if t := billing.Tier(sess.Metadata.Tier); t == billing.TierGrowth || t == billing.TierEnterprise {
		tier = t
	}

	_, _ = s.db.Exec(r.Context(),
		`INSERT INTO subscriptions (organisation_id, stripe_customer_id, stripe_sub_id, tier, status)
		 VALUES ($1,$2,$3,$4,'active')
		 ON CONFLICT (organisation_id)
		 DO UPDATE SET stripe_customer_id=$2, stripe_sub_id=$3, tier=$4, status='active', updated_at=now()`,
		sess.ClientReferenceID, sess.Customer, sess.Subscription, string(tier),
	)
}

// handleSubscriptionUpdate syncs subscription status changes from Stripe.
func (s *Server) handleSubscriptionUpdate(r *http.Request, raw json.RawMessage) {
	var sub struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		CancelAt *int64 `json:"cancel_at"`
		Items    struct {
			Data []struct {
				Price struct {
					ID string `json:"id"`
				} `json:"price"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &sub); err != nil || sub.ID == "" {
		return
	}

	// Map price ID → tier so upgrades/downgrades are reflected correctly.
	tier := billing.TierFree
	if len(sub.Items.Data) > 0 {
		tier = tierFromPriceID(sub.Items.Data[0].Price.ID)
	}

	cancelAtEnd := sub.CancelAt != nil
	_, _ = s.db.Exec(r.Context(),
		`UPDATE subscriptions
		 SET tier=$1, status=$2, cancel_at_period_end=$3, updated_at=now()
		 WHERE stripe_sub_id=$4`,
		string(tier), sub.Status, cancelAtEnd, sub.ID,
	)
}

// stripePriceID maps a tier name to its Stripe price ID (from env vars).
func stripePriceID(tier string) string {
	switch tier {
	case "starter":
		return os.Getenv("STRIPE_PRICE_STARTER")
	case "growth":
		return os.Getenv("STRIPE_PRICE_GROWTH")
	case "enterprise":
		return os.Getenv("STRIPE_PRICE_ENTERPRISE")
	}
	return ""
}

// tierFromPriceID maps a Stripe price ID back to a Tier.
func tierFromPriceID(priceID string) billing.Tier {
	switch priceID {
	case os.Getenv("STRIPE_PRICE_STARTER"):
		return billing.TierStarter
	case os.Getenv("STRIPE_PRICE_GROWTH"):
		return billing.TierGrowth
	case os.Getenv("STRIPE_PRICE_ENTERPRISE"):
		return billing.TierEnterprise
	}
	return billing.TierFree
}
