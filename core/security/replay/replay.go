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

package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	pathutil "path"
	"strings"
	"sync"
	"time"

	"infini.sh/framework/core/util"
)

const (
	HeaderName = "X-Request-Nonce"
	DefaultTTL = 30 * time.Second
)

type SubjectExtractor func(r *http.Request) string

type StoreOptions struct {
	TTL              time.Duration
	SubjectExtractor SubjectExtractor
}

type nonceRecord struct {
	Subject   string
	Method    string
	Path      string
	ExpiresAt time.Time
}

type Store struct {
	mu               sync.Mutex
	ttl              time.Duration
	subjectExtractor SubjectExtractor
	records          map[string]nonceRecord
}

var defaultStore = NewStore(StoreOptions{})

func NewStore(options StoreOptions) *Store {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	extractor := options.SubjectExtractor
	if extractor == nil {
		extractor = DefaultSubjectExtractor
	}
	return &Store{
		ttl:              ttl,
		subjectExtractor: extractor,
		records:          map[string]nonceRecord{},
	}
}

func IssueReplayNonce(r *http.Request, method, requestPath string) (string, time.Duration, error) {
	return defaultStore.IssueReplayNonce(r, method, requestPath)
}

func ValidateAndConsumeReplayNonce(r *http.Request) error {
	return defaultStore.ValidateAndConsumeReplayNonce(r)
}

func DefaultSubjectExtractor(r *http.Request) string {
	if r == nil {
		return "anonymous"
	}
	authorizationHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorizationHeader == "" {
		return "anonymous"
	}
	// Bind the nonce to the caller's authorization material so a replay token issued for one
	// authenticated context cannot be reused with a different credential set.
	sum := sha256.Sum256([]byte(authorizationHeader))
	return hex.EncodeToString(sum[:])
}

func (store *Store) IssueReplayNonce(r *http.Request, method, requestPath string) (string, time.Duration, error) {
	normalizedMethod, normalizedPath, err := normalizeScope(method, requestPath)
	if err != nil {
		return "", 0, err
	}

	nonce := util.GenerateSecureString(32)
	if nonce == "" {
		return "", 0, fmt.Errorf("failed to generate replay nonce")
	}

	subject := store.extractSubject(r)
	expiresAt := time.Now().Add(store.ttl)

	store.mu.Lock()
	defer store.mu.Unlock()
	store.cleanupExpiredLocked(time.Now())
	store.records[nonce] = nonceRecord{
		Subject:   subject,
		Method:    normalizedMethod,
		Path:      normalizedPath,
		ExpiresAt: expiresAt,
	}
	return nonce, store.ttl, nil
}

func (store *Store) ValidateAndConsumeReplayNonce(r *http.Request) error {
	if r == nil {
		return fmt.Errorf("request can not be nil")
	}

	nonce := strings.TrimSpace(r.Header.Get(HeaderName))
	if nonce == "" {
		return fmt.Errorf("missing replay nonce")
	}

	subject := store.extractSubject(r)
	method, requestPath, err := normalizeScope(r.Method, r.URL.Path)
	if err != nil {
		return err
	}

	now := time.Now()
	store.mu.Lock()
	defer store.mu.Unlock()
	store.cleanupExpiredLocked(now)

	record, ok := store.records[nonce]
	if !ok {
		return fmt.Errorf("replay nonce is invalid or expired")
	}
	delete(store.records, nonce)

	if record.Subject != subject || record.Method != method || record.Path != requestPath {
		return fmt.Errorf("replay nonce does not match request context")
	}

	return nil
}

func (store *Store) extractSubject(r *http.Request) string {
	if store == nil || store.subjectExtractor == nil {
		return DefaultSubjectExtractor(r)
	}
	return store.subjectExtractor(r)
}

func (store *Store) cleanupExpiredLocked(now time.Time) {
	for nonce, record := range store.records {
		if now.After(record.ExpiresAt) {
			delete(store.records, nonce)
		}
	}
}

func normalizeScope(method, requestPath string) (string, string, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	switch normalizedMethod {
	case http.MethodPost, http.MethodPut, http.MethodDelete:
	default:
		return "", "", fmt.Errorf("unsupported replay-protected method [%s]", method)
	}

	normalizedPath := strings.TrimSpace(requestPath)
	if normalizedPath == "" {
		return "", "", fmt.Errorf("request path can not be empty")
	}
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}
	normalizedPath = pathutil.Clean(normalizedPath)
	if normalizedPath == "." {
		normalizedPath = "/"
	}

	return normalizedMethod, normalizedPath, nil
}
