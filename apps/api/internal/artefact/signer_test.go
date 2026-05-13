package artefact_test

import (
	"testing"

	"github.com/YASSERRMD/Readinova/apps/api/internal/artefact"
)

func TestSignAndVerify(t *testing.T) {
	kp, err := artefact.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	payload := artefact.Payload{
		AssessmentID:    "test-assessment-id",
		OrganisationID:  "test-org-id",
		CompositeScore:  72.5,
		DimensionScores: map[string]float64{"strategy": 80, "data": 65},
		FrameworkVersion: "1.0",
		EngineVersion:    "0.1.0",
	}

	signed, err := artefact.Sign(payload, kp)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if err := artefact.Verify(signed); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyTamperedPayload(t *testing.T) {
	kp, _ := artefact.GenerateKeyPair()
	payload := artefact.Payload{
		AssessmentID:   "id",
		OrganisationID: "org",
		CompositeScore: 50,
	}
	signed, _ := artefact.Sign(payload, kp)

	// Tamper with the score.
	signed.Payload.CompositeScore = 99
	if err := artefact.Verify(signed); err == nil {
		t.Fatal("expected verification to fail after tampering")
	}
}

func TestVerifyTamperedSignature(t *testing.T) {
	kp, _ := artefact.GenerateKeyPair()
	payload := artefact.Payload{AssessmentID: "id", OrganisationID: "org"}
	signed, _ := artefact.Sign(payload, kp)

	// Corrupt the signature.
	signed.SignatureB64 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if err := artefact.Verify(signed); err == nil {
		t.Fatal("expected verification to fail with bad signature")
	}
}
