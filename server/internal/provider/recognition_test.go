package provider

import (
	"context"
	"testing"
)

func TestFakeProviderIsExplicitAndPartial(t *testing.T) {
	provider := Fake{}
	if provider.Name() != "fake:development" {
		t.Fatal("fake provider must identify itself")
	}
	result, err := provider.Recognize(context.Background(), "front", "image/jpeg", []byte("image"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "partial" || result.Fields["productName"] == "" {
		t.Fatalf("unexpected fake result: %+v", result)
	}
}

func TestExpiryRequiresVisibleEvidence(t *testing.T) {
	rejected, err := normalize("expiry", Candidate{Status: "recognized", Date: "2028-09-30", Raw: "LOT A113", Confidence: .9})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != "unrecognized" || rejected.Date != "" {
		t.Fatalf("unsupported expiry accepted: %+v", rejected)
	}
	accepted, err := normalize("expiry", Candidate{Status: "recognized", Date: "2028-09-30", Raw: "EXP 2028/09/30", Confidence: .9})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Date != "2028-09-30" {
		t.Fatalf("supported expiry rejected: %+v", accepted)
	}
}

func TestExpiryRejectsImpossibleCalendarDate(t *testing.T) {
	value, err := normalize("expiry", Candidate{Status: "recognized", Date: "2027-13-40", Raw: "EXP 2027-13-40", Confidence: .9})
	if err != nil {
		t.Fatal(err)
	}
	if value.Date != "" || value.Status != "unrecognized" {
		t.Fatalf("impossible date must be rejected: %+v", value)
	}
}
