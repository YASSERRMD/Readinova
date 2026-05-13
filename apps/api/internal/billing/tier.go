package billing

// Tier represents a subscription tier.
type Tier string

const (
	TierFree       Tier = "free"
	TierStarter    Tier = "starter"
	TierGrowth     Tier = "growth"
	TierEnterprise Tier = "enterprise"
)

// TierLimits defines feature limits per tier.
type TierLimits struct {
	MaxAssessments       int // -1 = unlimited
	MaxTeamMembers       int // -1 = unlimited
	PDFWatermark         bool
	EvidenceConnectors   bool
	RecommendationEngine bool
	AuditArtefacts       bool
}

var tierLimits = map[Tier]TierLimits{
	TierFree: {
		MaxAssessments:       1,
		MaxTeamMembers:       3,
		PDFWatermark:         true,
		EvidenceConnectors:   false,
		RecommendationEngine: false,
		AuditArtefacts:       false,
	},
	TierStarter: {
		MaxAssessments:       5,
		MaxTeamMembers:       10,
		PDFWatermark:         false,
		EvidenceConnectors:   false,
		RecommendationEngine: true,
		AuditArtefacts:       false,
	},
	TierGrowth: {
		MaxAssessments:       -1,
		MaxTeamMembers:       50,
		PDFWatermark:         false,
		EvidenceConnectors:   true,
		RecommendationEngine: true,
		AuditArtefacts:       true,
	},
	TierEnterprise: {
		MaxAssessments:       -1,
		MaxTeamMembers:       -1,
		PDFWatermark:         false,
		EvidenceConnectors:   true,
		RecommendationEngine: true,
		AuditArtefacts:       true,
	},
}

// LimitsFor returns the feature limits for a tier.
func LimitsFor(t Tier) TierLimits {
	if l, ok := tierLimits[t]; ok {
		return l
	}
	return tierLimits[TierFree]
}
