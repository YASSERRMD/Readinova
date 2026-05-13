// Package connector defines the interface for evidence collection connectors.
package connector

import "context"

// Signal is a single evidence datum collected from a connector.
type Signal struct {
	DimensionSlug string
	SignalKey     string
	SignalValue   any // must be JSON-serialisable
}

// Connector is the contract every evidence source must satisfy.
type Connector interface {
	// Type returns the unique string identifier for this connector (e.g. "azure").
	Type() string

	// Connect validates credentials and opens any long-lived connections.
	Connect(ctx context.Context, credentials map[string]any) error

	// Collect gathers evidence signals for the given dimension slugs.
	// Pass nil to collect for all supported dimensions.
	Collect(ctx context.Context, dimensions []string) ([]Signal, error)

	// Disconnect releases any open connections and resources.
	Disconnect(ctx context.Context) error
}

// Registry holds all registered connector factories.
var Registry = map[string]func() Connector{}

// Register adds a connector factory to the global registry.
func Register(connType string, factory func() Connector) {
	Registry[connType] = factory
}
