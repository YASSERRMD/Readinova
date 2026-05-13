package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func init() {
	Register("azure", func() Connector { return &AzureConnector{} })
}

// AzureConnector collects read-only signals from Azure ARM and Microsoft Graph.
// Required credentials: tenant_id, client_id, client_secret, subscription_id.
type AzureConnector struct {
	tenantID       string
	subscriptionID string
	armToken       string  // token for management.azure.com
	graphToken     string  // token for graph.microsoft.com
	httpClient     *http.Client
}

func (c *AzureConnector) Type() string { return "azure" }

func (c *AzureConnector) Connect(ctx context.Context, creds map[string]any) error {
	tenantID, _ := creds["tenant_id"].(string)
	clientID, _ := creds["client_id"].(string)
	clientSecret, _ := creds["client_secret"].(string)
	subID, _ := creds["subscription_id"].(string)

	if tenantID == "" || clientID == "" || clientSecret == "" {
		return fmt.Errorf("azure connector requires tenant_id, client_id, client_secret")
	}

	c.tenantID = tenantID
	c.subscriptionID = subID
	c.httpClient = &http.Client{Timeout: 30 * time.Second}

	var err error
	c.armToken, err = c.fetchToken(ctx, tenantID, clientID, clientSecret, "https://management.azure.com/.default")
	if err != nil {
		return fmt.Errorf("ARM token: %w", err)
	}
	c.graphToken, err = c.fetchToken(ctx, tenantID, clientID, clientSecret, "https://graph.microsoft.com/.default")
	if err != nil {
		return fmt.Errorf("Graph token: %w", err)
	}
	return nil
}

func (c *AzureConnector) Collect(ctx context.Context, dimensions []string) ([]Signal, error) {
	if c.armToken == "" {
		return nil, fmt.Errorf("connector not connected")
	}

	wanted := map[string]bool{}
	if len(dimensions) > 0 {
		for _, d := range dimensions {
			wanted[d] = true
		}
	}

	var signals []Signal

	if len(dimensions) == 0 || wanted["technology_infrastructure"] {
		sigs, err := c.collectARMSignals(ctx)
		if err == nil {
			signals = append(signals, sigs...)
		}
	}

	if len(dimensions) == 0 || wanted["data_governance"] || wanted["ethics_governance"] {
		sigs, err := c.collectGraphSignals(ctx)
		if err == nil {
			signals = append(signals, sigs...)
		}
	}

	return signals, nil
}

func (c *AzureConnector) Disconnect(_ context.Context) error {
	c.armToken = ""
	c.graphToken = ""
	return nil
}

// collectARMSignals collects ARM-based signals.
func (c *AzureConnector) collectARMSignals(ctx context.Context) ([]Signal, error) {
	var signals []Signal

	// Subscriptions list.
	resp, err := c.armGET(ctx, "https://management.azure.com/subscriptions?api-version=2022-12-01")
	if err == nil {
		var body struct {
			Value []struct{ ID string `json:"id"` } `json:"value"`
		}
		if json.Unmarshal(resp, &body) == nil {
			signals = append(signals, Signal{
				DimensionSlug: "technology_infrastructure",
				SignalKey:     "azure_subscription_count",
				SignalValue:   len(body.Value),
			})
		}
	}

	// Policy compliance state (if subscription_id provided).
	if c.subscriptionID != "" {
		policyURL := fmt.Sprintf(
			"https://management.azure.com/subscriptions/%s/providers/Microsoft.PolicyInsights/policyStates/latest/summarize?api-version=2019-10-01",
			c.subscriptionID,
		)
		resp, err = c.armGET(ctx, policyURL)
		if err == nil {
			var body struct {
				Value []struct {
					Results struct {
						NonCompliantResources int `json:"nonCompliantResources"`
						ResourceDetails       []struct {
							ComplianceState string `json:"complianceState"`
							Count           int    `json:"count"`
						} `json:"resourceDetails"`
					} `json:"results"`
				} `json:"value"`
			}
			if json.Unmarshal(resp, &body) == nil && len(body.Value) > 0 {
				nonCompliant := body.Value[0].Results.NonCompliantResources
				signals = append(signals, Signal{
					DimensionSlug: "ethics_governance",
					SignalKey:     "azure_policy_non_compliant_resources",
					SignalValue:   nonCompliant,
				})
			}
		}

		// Resource count by type.
		resourceURL := fmt.Sprintf(
			"https://management.azure.com/subscriptions/%s/resources?api-version=2021-04-01&$top=1",
			c.subscriptionID,
		)
		resp, err = c.armGET(ctx, resourceURL)
		if err == nil {
			var body struct {
				Value []json.RawMessage `json:"value"`
			}
			if json.Unmarshal(resp, &body) == nil {
				signals = append(signals, Signal{
					DimensionSlug: "technology_infrastructure",
					SignalKey:     "azure_resource_count_sample",
					SignalValue:   len(body.Value),
				})
			}
		}
	}

	return signals, nil
}

// collectGraphSignals collects Microsoft Graph-based signals.
func (c *AzureConnector) collectGraphSignals(ctx context.Context) ([]Signal, error) {
	var signals []Signal

	// User count.
	resp, err := c.graphGET(ctx, "https://graph.microsoft.com/v1.0/users?$top=1&$count=true&ConsistencyLevel=eventual")
	if err == nil {
		var body struct {
			OdataCount int `json:"@odata.count"`
		}
		if json.Unmarshal(resp, &body) == nil && body.OdataCount > 0 {
			signals = append(signals, Signal{
				DimensionSlug: "talent_culture",
				SignalKey:     "azure_ad_user_count",
				SignalValue:   body.OdataCount,
			})
		}
	}

	// Service principal count (apps registered).
	resp, err = c.graphGET(ctx, "https://graph.microsoft.com/v1.0/servicePrincipals?$top=1&$count=true&ConsistencyLevel=eventual")
	if err == nil {
		var body struct {
			OdataCount int `json:"@odata.count"`
		}
		if json.Unmarshal(resp, &body) == nil {
			signals = append(signals, Signal{
				DimensionSlug: "technology_infrastructure",
				SignalKey:     "azure_service_principal_count",
				SignalValue:   body.OdataCount,
			})
		}
	}

	// Conditional access policies (governance signal).
	resp, err = c.graphGET(ctx, "https://graph.microsoft.com/v1.0/identity/conditionalAccess/policies")
	if err == nil {
		var body struct {
			Value []struct {
				State string `json:"state"`
			} `json:"value"`
		}
		if json.Unmarshal(resp, &body) == nil {
			enabled := 0
			for _, p := range body.Value {
				if p.State == "enabled" {
					enabled++
				}
			}
			signals = append(signals, Signal{
				DimensionSlug: "ethics_governance",
				SignalKey:     "azure_conditional_access_policies_enabled",
				SignalValue:   enabled,
			})
		}
	}

	// Groups count.
	resp, err = c.graphGET(ctx, "https://graph.microsoft.com/v1.0/groups?$top=1&$count=true&ConsistencyLevel=eventual")
	if err == nil {
		var body struct {
			OdataCount int `json:"@odata.count"`
		}
		if json.Unmarshal(resp, &body) == nil {
			signals = append(signals, Signal{
				DimensionSlug: "data_governance",
				SignalKey:     "azure_ad_group_count",
				SignalValue:   body.OdataCount,
			})
		}
	}

	return signals, nil
}

func (c *AzureConnector) armGET(ctx context.Context, endpoint string) ([]byte, error) {
	return c.doGET(ctx, endpoint, c.armToken)
}

func (c *AzureConnector) graphGET(ctx context.Context, endpoint string) ([]byte, error) {
	return c.doGET(ctx, endpoint, c.graphToken)
}

func (c *AzureConnector) doGET(ctx context.Context, endpoint, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *AzureConnector) fetchToken(ctx context.Context, tenantID, clientID, clientSecret, scope string) (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("token parse: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}
	return result.AccessToken, nil
}
