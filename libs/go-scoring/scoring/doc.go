// Package scoring wraps the Rust scoring cdylib via cgo.
//
// Usage:
//
//	result, err := scoring.Score(ctx, framework, responses)
//
// The Rust cdylib must be built and placed in libs/go-scoring/lib/ before
// building or testing this package. Use `make scoring` from the repo root.
package scoring
