/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"reflect"
	"testing"
)

func TestDenyRoleAssignment(t *testing.T) {
	cases := []struct {
		name        string
		callerRoles []string
		requested   []string
		want        []string
	}{
		{"admin can grant anything", []string{"admin"}, []string{"admin", "readonly", "editor"}, nil},
		{"holder can grant held role", []string{"editor"}, []string{"editor"}, []string{}},
		{"non-holder cannot grant", []string{"editor"}, []string{"admin"}, []string{"admin"}},
		{
			"mixed request returns only denied subset",
			[]string{"editor", "viewer"},
			[]string{"viewer", "admin", "editor"},
			[]string{"admin"},
		},
		{"empty caller denies all", nil, []string{"viewer"}, []string{"viewer"}},
		{"empty request allows", []string{"viewer"}, nil, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DenyRoleAssignment(c.callerRoles, c.requested)
			if c.want == nil && got != nil {
				t.Fatalf("expected nil (all allowed), got %v", got)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("expected %v, got %v", c.want, got)
			}
		})
	}
}
