package passwordchallenge

import "testing"

func TestPasswordChallengeProofRoundTrip(t *testing.T) {
	verifier, err := DeriveVerifier("admin", "salt-123")
	if err != nil {
		t.Fatalf("derive verifier: %v", err)
	}

	challenge := New("admin")
	proof, err := BuildProof(verifier, "admin", challenge.ID, challenge.Nonce)
	if err != nil {
		t.Fatalf("build proof: %v", err)
	}

	if !VerifyProof(verifier, "admin", challenge.ID, challenge.Nonce, proof) {
		t.Fatal("expected password proof to validate")
	}
}

func TestConsumeRejectsWrongSubject(t *testing.T) {
	store := NewStore(StoreOptions{})
	challenge := store.New("admin")

	if _, err := store.Consume(challenge.ID, "guest"); err == nil {
		t.Fatal("expected challenge subject mismatch to fail")
	}
}
