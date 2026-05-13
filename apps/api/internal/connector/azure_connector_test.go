package connector_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/Readinova/apps/api/internal/connector"
)

func TestAzureConnectorType(t *testing.T) {
	factory, ok := connector.Registry["azure"]
	if !ok {
		t.Fatal("azure connector not registered")
	}
	c := factory()
	if c.Type() != "azure" {
		t.Fatalf("expected type 'azure', got %q", c.Type())
	}
}

func TestAzureConnectorMissingCreds(t *testing.T) {
	factory := connector.Registry["azure"]
	c := factory()
	err := c.Connect(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestAzureConnectorNotConnected(t *testing.T) {
	factory := connector.Registry["azure"]
	c := factory()
	_, err := c.Collect(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when collecting without connecting")
	}
}
