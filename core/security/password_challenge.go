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
	PasswordChallengeMethod     = passwordchallenge.Method
	PasswordChallengeAlgorithm  = passwordchallenge.Algorithm
	PasswordChallengeIterations = passwordchallenge.Iterations
)

type LoginChallenge = passwordchallenge.Challenge

func CanUsePasswordChallenge(user *UserAccount) bool {
	return user != nil && user.PasswordSalt != "" && user.PasswordVerifier != ""
}

func SetPassword(user *UserAccount, password string) error {
	if user == nil {
		return errors.New("user is nil")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	salt := util.GenerateSecureString(32)
	verifier, err := DerivePasswordVerifier(password, salt)
	if err != nil {
		return err
	}

	user.Password = string(hash)
	user.PasswordSalt = salt
	user.PasswordVerifier = verifier
	return nil
}

func EnsurePasswordChallenge(user *UserAccount, password string) error {
	if user == nil {
		return errors.New("user is nil")
	}
	if CanUsePasswordChallenge(user) {
		return nil
	}

	salt := util.GenerateSecureString(32)
	verifier, err := DerivePasswordVerifier(password, salt)
	if err != nil {
		return err
	}

	user.PasswordSalt = salt
	user.PasswordVerifier = verifier
	return nil
}

func VerifyPassword(user *UserAccount, password string) error {
	if user == nil {
		return errors.New("user is nil")
	}
	if user.Password == "" {
		return errors.New("password is not set")
	}
	return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
}

func DerivePasswordVerifier(password, salt string) (string, error) {
	return passwordchallenge.DeriveVerifier(password, salt)
}

func BuildPasswordProof(verifier, subject, challengeID, nonce string) (string, error) {
	return passwordchallenge.BuildProof(verifier, subject, challengeID, nonce)
}

func VerifyPasswordProof(verifier, subject, challengeID, nonce, proof string) bool {
	return passwordchallenge.VerifyProof(verifier, subject, challengeID, nonce, proof)
}

func NewLoginChallenge(subject string) LoginChallenge {
	return passwordchallenge.New(subject)
}

func ConsumeLoginChallenge(challengeID, subject string) (LoginChallenge, error) {
	return passwordchallenge.Consume(challengeID, subject)
}
