/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package team groups accounts by typed ID or login references.
// Login stores the verified account's team IDs in its session for sharing checks.
package team

import (
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
)

// MemberKind identifies the account field referenced by a team member.
//
// IDs and logins have separate namespaces. The kind prevents a value in
// one field from matching another account's value in the other field.
type MemberKind string

const (
	MemberKindUserID MemberKind = "user_id"
	MemberKindLogin  MemberKind = "login"
)

// MemberId references an account by ID or login.
type MemberId struct {
	Kind  MemberKind `json:"kind" elastic_mapping:"kind: {type: keyword}"`
	Value string     `json:"value" elastic_mapping:"value: {type: keyword}"`
}

// Team groups accounts by ID or login. Both fields of a member must match
// within the same array element, so the storage mapping uses nested objects.
type Team struct {
	orm.ORMObjectBase
	Name        string     `json:"name" elastic_mapping:"name: {type: keyword}"`
	Description string     `json:"description" elastic_mapping:"description: {type: text}"`
	Members     []MemberId `json:"members" elastic_mapping:"members: {type: nested}"`
}

func (t *Team) GetIndexName() string { return "security-team" }

// List returns all teams, paging through the index so the result never
// silently truncates at a single page.
func List(ctx *orm.Context) ([]Team, error) {
	if !orm.IsHandlerRegistered() {
		return []Team{}, nil
	}
	orm.WithModel(ctx, &Team{})
	const pageSize = 500
	var teams []Team
	for from := 0; ; from += pageSize {
		res, err := orm.SearchV2(ctx, orm.NewQuery().From(from).Size(pageSize))
		if err != nil {
			return nil, err
		}
		page, _, err := elastic.DecodeHits[Team](res)
		if err != nil {
			return nil, err
		}
		teams = append(teams, page...)
		if len(page) < pageSize {
			break
		}
	}
	return teams, nil
}

// TeamIDsForUser resolves membership from a verified account's ID and login.
//
// Storage errors yield no memberships and do not prevent password login.
// Page lengths, rather than possibly approximate totals, determine completion.
func TeamIDsForUser(userID, login string) []string {
	if !orm.IsHandlerRegistered() || (userID == "" && login == "") {
		return nil
	}
	ctx := orm.NewContext().DirectReadAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)
	orm.WithModel(ctx, &Team{})

	var conditions []*orm.Clause
	for _, member := range []MemberId{{MemberKindUserID, userID}, {MemberKindLogin, login}} {
		if member.Value != "" {
			conditions = append(conditions, orm.NestedQuery("members", orm.MustQuery(
				orm.TermQuery("members.kind", string(member.Kind)),
				orm.TermQuery("members.value", member.Value))))
		}
	}
	const pageSize = 500
	var ids []string
	seen := map[string]struct{}{}
	for from := 0; ; from += pageSize {
		res, err := orm.SearchV2(ctx, orm.NewQuery().Should(conditions...).MinimumShouldMatch(1).From(from).Size(pageSize))
		if err != nil {
			return nil
		}
		teams, _, err := elastic.DecodeHits[Team](res)
		if err != nil {
			return nil
		}
		for _, team := range teams {
			if _, duplicate := seen[team.ID]; duplicate {
				continue
			}
			for _, member := range team.Members {
				if memberMatches(member, userID, login) {
					seen[team.ID] = struct{}{}
					ids = append(ids, team.ID)
					break
				}
			}
		}
		if len(teams) < pageSize {
			return ids
		}
	}
}

// Helper function to compare a stored member against verified account fields.
func memberMatches(member MemberId, userID, login string) bool {
	if member.Value == "" {
		return false
	}
	switch member.Kind {
	case MemberKindUserID:
		return member.Value == userID
	case MemberKindLogin:
		return member.Value == login
	default:
		return false
	}
}
