// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package passwordchallenge

import (
	"testing"
	"time"
)

// Proof generation and verification need to round-trip because this package defines the
// wire contract shared between the framework login endpoint and upgraded clients.
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

// Challenges are bound to the requested login subject and must not be replayed for others.
func TestConsumeRejectsWrongSubject(t *testing.T) {
	store := NewStore(StoreOptions{})
	challenge := store.New("admin")

	if _, err := store.Consume(challenge.ID, "guest"); err == nil {
		t.Fatal("expected challenge subject mismatch to fail")
	}
}

// Empty input should be rejected up front to avoid persisting or comparing invalid verifiers.
func TestDeriveVerifierRejectsEmptyInput(t *testing.T) {
	if _, err := DeriveVerifier("", "salt-123"); err == nil {
		t.Fatal("expected empty password to fail")
	}
	if _, err := DeriveVerifier("admin", ""); err == nil {
		t.Fatal("expected empty salt to fail")
	}
}

// Expiration keeps the one-time challenge store bounded and prevents stale proof reuse.
func TestConsumeRejectsExpiredChallenge(t *testing.T) {
	store := NewStore(StoreOptions{TTL: time.Millisecond})
	challenge := store.New("admin")

	time.Sleep(5 * time.Millisecond)
	if _, err := store.Consume(challenge.ID, "admin"); err == nil {
		t.Fatal("expected expired challenge to fail")
	}
}
