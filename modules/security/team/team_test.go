/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package team

import "testing"

// Membership entries reference a user by user ID or any login alias: the
// submitted login (which may be an email) and the account's canonical name
// (the form members are typically listed under).
func TestMemberMatches(t *testing.T) {
	const uid = "usr_123"

	if !memberMatches(uid, uid, nil) {
		t.Fatal("user ID must match itself")
	}
	if !memberMatches("medcl", uid, []string{"medcl@example.com", "medcl"}) {
		t.Fatal("account name alias must match")
	}
	if !memberMatches("medcl@example.com", uid, []string{"medcl@example.com"}) {
		t.Fatal("submitted email alias must match")
	}
	if memberMatches("other", uid, []string{"medcl"}) {
		t.Fatal("unrelated member must not match")
	}
	if memberMatches(uid, "", []string{""}) {
		t.Fatal("empty identities must never match")
	}
}
