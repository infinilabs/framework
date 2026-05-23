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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"infini.sh/framework/core/util"
)

const (
	Method     = "challenge"
	Algorithm  = "PBKDF2-SHA256"
	Iterations = 120000
	keyLength  = 32
	DefaultTTL = 5 * time.Minute
)

type Challenge struct {
	ID       string
	Subject  string
	Nonce    string
	ExpireAt time.Time
}

type StoreOptions struct {
	TTL time.Duration
}

type Store struct {
	mu         sync.Mutex
	ttl        time.Duration
	challenges map[string]Challenge
}

var defaultStore = NewStore(StoreOptions{})

func NewStore(options StoreOptions) *Store {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Store{
		ttl:        ttl,
		challenges: map[string]Challenge{},
	}
}

func DeriveVerifier(password, salt string) (string, error) {
	if password == "" {
		return "", errors.New("password is empty")
	}
	if salt == "" {
		return "", errors.New("password salt is empty")
	}
	key := pbkdf2.Key([]byte(password), []byte(salt), Iterations, keyLength, sha256.New)
	return hex.EncodeToString(key), nil
}

func BuildProof(verifier, subject, challengeID, nonce string) (string, error) {
	key, err := hex.DecodeString(verifier)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(subject))
	mac.Write([]byte(":"))
	mac.Write([]byte(challengeID))
	mac.Write([]byte(":"))
	mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func VerifyProof(verifier, subject, challengeID, nonce, proof string) bool {
	expected, err := BuildProof(verifier, subject, challengeID, nonce)
	if err != nil {
		return false
	}
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	proofBytes, err := hex.DecodeString(strings.ToLower(proof))
	if err != nil {
		return false
	}
	return hmac.Equal(expectedBytes, proofBytes)
}

func New(subject string) Challenge {
	return defaultStore.New(subject)
}

func Consume(challengeID, subject string) (Challenge, error) {
	return defaultStore.Consume(challengeID, subject)
}

func (store *Store) New(subject string) Challenge {
	now := time.Now()
	store.mu.Lock()
	defer store.mu.Unlock()

	for id, challenge := range store.challenges {
		if challenge.ExpireAt.Before(now) {
			delete(store.challenges, id)
		}
	}

	challenge := Challenge{
		ID:       util.GenerateSecureString(32),
		Subject:  subject,
		Nonce:    util.GenerateSecureString(32),
		ExpireAt: now.Add(store.ttl),
	}
	store.challenges[challenge.ID] = challenge
	return challenge
}

func (store *Store) Consume(challengeID, subject string) (Challenge, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	challenge, ok := store.challenges[challengeID]
	if !ok {
		return Challenge{}, errors.New("login challenge is invalid")
	}
	delete(store.challenges, challengeID)

	if challenge.ExpireAt.Before(time.Now()) {
		return Challenge{}, errors.New("login challenge expired")
	}
	if challenge.Subject != subject {
		return Challenge{}, errors.New("login challenge does not match user")
	}
	return challenge, nil
}
