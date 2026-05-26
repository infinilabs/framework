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

package security

import "testing"

func TestSetPasswordPopulatesChallengeFields(t *testing.T) {
	user := &UserAccount{}
	if err := SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	if user.Password == "" {
		t.Fatal("expected password hash to be set")
	}
	if user.PasswordSalt == "" {
		t.Fatal("expected password salt to be set")
	}
	if user.PasswordVerifier == "" {
		t.Fatal("expected password verifier to be set")
	}
	if err := VerifyPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("verify password: %v", err)
	}
}

func TestEnsurePasswordChallengePreservesExistingVerifier(t *testing.T) {
	user := &UserAccount{}
	if err := SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	originalSalt := user.PasswordSalt
	originalVerifier := user.PasswordVerifier
	if err := EnsurePasswordChallenge(user, "AnotherStrongPassw0rd!"); err != nil {
		t.Fatalf("ensure password challenge: %v", err)
	}

	if user.PasswordSalt != originalSalt {
		t.Fatal("expected existing password salt to be preserved")
	}
	if user.PasswordVerifier != originalVerifier {
		t.Fatal("expected existing password verifier to be preserved")
	}
}

func TestPasswordChallengeProofRoundTrip(t *testing.T) {
	user := &UserAccount{}
	login := "admin@example.org"
	password := "StrongPassw0rd!"

	if err := SetPassword(user, password); err != nil {
		t.Fatalf("set password: %v", err)
	}

	challenge := NewLoginChallenge(login)
	proof, err := BuildPasswordProof(user.PasswordVerifier, login, challenge.ID, challenge.Nonce)
	if err != nil {
		t.Fatalf("build password proof: %v", err)
	}

	if !VerifyPasswordProof(user.PasswordVerifier, login, challenge.ID, challenge.Nonce, proof) {
		t.Fatal("expected challenge proof to validate")
	}
}

func TestEnsurePasswordChallengePopulatesLegacyAccount(t *testing.T) {
	user := &UserAccount{Password: "existing-bcrypt-hash"}
	if err := EnsurePasswordChallenge(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("ensure password challenge: %v", err)
	}

	if user.PasswordSalt == "" {
		t.Fatal("expected password salt to be populated")
	}
	if user.PasswordVerifier == "" {
		t.Fatal("expected password verifier to be populated")
	}
	if !CanUsePasswordChallenge(user) {
		t.Fatal("expected legacy account to become challenge-capable")
	}
}
