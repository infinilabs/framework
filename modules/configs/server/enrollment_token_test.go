/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"testing"
	"time"

	httptest "net/http/httptest"
)

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(time.Minute, 3)
	ip := "10.0.0.1"
	for i := 0; i < 3; i++ {
		if !rl.allow(ip) {
			t.Fatalf("hit %d should be allowed (max 3)", i+1)
		}
	}
	if rl.allow(ip) {
		t.Fatal("4th hit in the same window must be denied")
	}
	// different key unaffected
	if !rl.allow("10.0.0.2") {
		t.Fatal("different IP must not share the bucket")
	}
	// window rollover
	rl.hits[ip].since = time.Now().Add(-2 * time.Minute)
	if !rl.allow(ip) {
		t.Fatal("new window must reset the bucket")
	}
	// disabled limiter
	var off *rateLimiter
	if !off.allow(ip) {
		t.Fatal("nil limiter = disabled = always allow")
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("POST", "/x", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	if got := clientIP(req); got != "1.2.3.4" {
		t.Fatalf("remote addr parse = %q", got)
	}
	req.Header.Set("X-Forwarded-For", "9.9.9.9, 10.0.0.1")
	if got := clientIP(req); got != "9.9.9.9" {
		t.Fatalf("proxy chain = %q", got)
	}
}

func TestEnrollmentTokenMint(t *testing.T) {
	rec, plaintext, err := mintEnrollmentToken("rollout", 5, time.Hour, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) <= len(enrollmentTokenPrefix) || plaintext[:len(enrollmentTokenPrefix)] != enrollmentTokenPrefix {
		t.Fatalf("plaintext = %q, want %s-... prefix", plaintext, enrollmentTokenPrefix)
	}
	if rec.TokenHash == plaintext {
		t.Fatal("hash must differ from plaintext")
	}
	if rec.MaxUses != 5 || rec.UsedCount != 0 {
		t.Fatalf("policy = %d/%d", rec.UsedCount, rec.MaxUses)
	}
	// deterministic hash of the same plaintext
	rec2, p2, _ := mintEnrollmentToken("x", 1, time.Hour, "")
	if rec2.TokenHash == rec.TokenHash && plaintext != p2 {
		t.Fatal("different plaintexts must not share a hash")
	}
}
