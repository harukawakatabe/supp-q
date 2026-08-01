package identity

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hashPassword returned error: %v", err)
	}
	if !verifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("expected password to verify")
	}
	if verifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password must not verify")
	}
}

func TestPasswordPolicy(t *testing.T) {
	if _, err := hashPassword("short"); err == nil {
		t.Fatal("expected short password rejection")
	}
}

func TestDigestsArePurposeSeparated(t *testing.T) {
	left := digest("pepper", "session", "same-value")
	right := digest("pepper", "invitation", "same-value")
	if left == right {
		t.Fatal("different token purposes must not share a digest")
	}
}
