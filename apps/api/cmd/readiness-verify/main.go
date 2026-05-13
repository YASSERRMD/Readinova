// readiness-verify verifies a signed Readinova audit artefact offline.
//
// Usage:
//
//	readiness-verify -f artefact.json
//	readiness-verify -f artefact.json -public-key <base64-pub-key>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/YASSERRMD/Readinova/apps/api/internal/artefact"
)

func main() {
	filePath := flag.String("f", "", "path to artefact JSON file (required)")
	expectedKey := flag.String("public-key", "", "expected base64 Ed25519 public key (optional additional check)")
	flag.Parse()

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "error: -f <artefact.json> is required")
		flag.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
		os.Exit(1)
	}

	var signed artefact.SignedArtefact
	if err := json.Unmarshal(data, &signed); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing artefact JSON: %v\n", err)
		os.Exit(1)
	}

	if err := artefact.Verify(&signed); err != nil {
		fmt.Fprintf(os.Stderr, "INVALID: %v\n", err)
		os.Exit(2)
	}

	if *expectedKey != "" && signed.PublicKeyB64 != *expectedKey {
		fmt.Fprintf(os.Stderr, "INVALID: public key mismatch\n  expected: %s\n  found:    %s\n",
			*expectedKey, signed.PublicKeyB64)
		os.Exit(2)
	}

	fmt.Println("VALID")
	fmt.Printf("  Assessment:     %s\n", signed.Payload.AssessmentID)
	fmt.Printf("  Organisation:   %s\n", signed.Payload.OrganisationID)
	fmt.Printf("  Composite Score: %.2f\n", signed.Payload.CompositeScore)
	fmt.Printf("  Signed At:      %s\n", signed.Payload.SignedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("  Payload Hash:   %s\n", signed.PayloadHash)
	fmt.Printf("  Public Key:     %s\n", signed.PublicKeyB64)
}
