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

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
	passwordchallenge "infini.sh/framework/core/security/passwordchallenge"
	"infini.sh/framework/core/util"
)

const (
	// PasswordChallengeMethod identifies the login flow returned by the challenge endpoint.
	PasswordChallengeMethod = passwordchallenge.Method
	// PasswordChallengeAlgorithm describes the verifier/proof derivation algorithm for clients.
	PasswordChallengeAlgorithm = passwordchallenge.Algorithm
	// PasswordChallengeIterations tells clients which PBKDF2 work factor to use.
	PasswordChallengeIterations = passwordchallenge.Iterations
)

// LoginChallenge re-exports the framework challenge payload used by native account login.
type LoginChallenge = passwordchallenge.Challenge

// PasswordMaterial bundles the fields that apps need to persist after accepting a password.
type PasswordMaterial struct {
	Hash     string
	Salt     string
	Verifier string
}

// CanUsePasswordChallenge reports whether a native account already has challenge credentials.
func CanUsePasswordChallenge(user *UserAccount) bool {
	return user != nil && user.PasswordSalt != "" && user.PasswordVerifier != ""
}

// GeneratePasswordMaterial derives the bcrypt hash and challenge verifier fields for a password.
func GeneratePasswordMaterial(password string) (*PasswordMaterial, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Store both the bcrypt hash for existing password checks and the derived
	// verifier for challenge login so the two login modes stay in sync.
	salt := util.GenerateSecureString(32)
	verifier, err := DerivePasswordVerifier(password, salt)
	if err != nil {
		return nil, err
	}

	return &PasswordMaterial{
		Hash:     string(hash),
		Salt:     salt,
		Verifier: verifier,
	}, nil
}

// SetPassword updates both the legacy bcrypt hash and the challenge verifier material.
func SetPassword(user *UserAccount, password string) error {
	if user == nil {
		return errors.New("user is nil")
	}

	material, err := GeneratePasswordMaterial(password)
	if err != nil {
		return err
	}

	user.Password = material.Hash
	user.PasswordSalt = material.Salt
	user.PasswordVerifier = material.Verifier
	return nil
}

// EnsurePasswordChallenge derives challenge material for older accounts without changing the bcrypt hash.
func EnsurePasswordChallenge(user *UserAccount, password string) error {
	if user == nil {
		return errors.New("user is nil")
	}
	if CanUsePasswordChallenge(user) {
		return nil
	}

	// This is used as an in-place upgrade path for older native accounts that only
	// have a bcrypt password hash from before challenge login was introduced.
	salt := util.GenerateSecureString(32)
	verifier, err := DerivePasswordVerifier(password, salt)
	if err != nil {
		return err
	}

	user.PasswordSalt = salt
	user.PasswordVerifier = verifier
	return nil
}

// VerifyPassword validates the plain password against the stored bcrypt hash.
func VerifyPassword(user *UserAccount, password string) error {
	if user == nil {
		return errors.New("user is nil")
	}
	if user.Password == "" {
		return errors.New("password is not set")
	}
	return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
}

// DerivePasswordVerifier converts a password and salt into the stored challenge verifier.
func DerivePasswordVerifier(password, salt string) (string, error) {
	return passwordchallenge.DeriveVerifier(password, salt)
}

// BuildPasswordProof creates the challenge response that clients send to /account/login.
func BuildPasswordProof(verifier, subject, challengeID, nonce string) (string, error) {
	return passwordchallenge.BuildProof(verifier, subject, challengeID, nonce)
}

// VerifyPasswordProof checks whether a submitted proof matches the stored verifier.
func VerifyPasswordProof(verifier, subject, challengeID, nonce, proof string) bool {
	return passwordchallenge.VerifyProof(verifier, subject, challengeID, nonce, proof)
}

// NewLoginChallenge allocates a one-time challenge bound to the requested login subject.
func NewLoginChallenge(subject string) LoginChallenge {
	return passwordchallenge.New(subject)
}

// ConsumeLoginChallenge validates and removes a one-time challenge after it is used.
func ConsumeLoginChallenge(challengeID, subject string) (LoginChallenge, error) {
	return passwordchallenge.Consume(challengeID, subject)
}
