/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package team provides the framework-generic security team: a principal
// that groups multiple users (matched by user ID, login name or account
// name — static accounts authenticate by either) so access can be granted
// to the group as a whole.
//
// Login resolves the verified account's memberships via TeamIDsForUser and
// carries them on the session under orm.TeamsIDKey — exactly where the share
// module expands principal_type=team rules to their members.
package team

import (
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
)

// Team is a named group of users. Members holds user IDs and/or login names
// (static accounts may be added by either form); matching accepts both.
type Team struct {
	orm.ORMObjectBase

	Name        string   `json:"name" elastic_mapping:"name: {type: keyword}"`
	Description string   `json:"description" elastic_mapping:"description: {type: text}"`
	Members     []string `json:"members" elastic_mapping:"members: {type: keyword}"`
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
		page, total, err := elastic.DecodeHits[Team](res)
		if err != nil {
			return nil, err
		}
		teams = append(teams, page...)
		if len(page) < pageSize || int64(from+len(page)) >= total {
			break
		}
	}
	return teams, nil
}

// TeamIDsForUser returns the IDs of the teams whose membership lists the
// given user — matched against the user ID plus any login alias (the
// submitted login and the account's canonical name: members may have been
// added by either form, and the submitted login can be an email while the
// member list holds the name). Used at login to populate the session's
// teams_id.
//
// Membership is matched server-side (terms query on the members keyword
// array) and paged through: a capped global scan would silently drop the
// memberships of teams beyond the first page. Runs on a background
// context: it must not fail logins, so any storage error simply yields no
// teams.
func TeamIDsForUser(userID string, aliases ...string) []string {
	// static-auth-only deployments run no ORM backend; the lookup below
	// would panic on it and take every password login down with it
	if !orm.IsHandlerRegistered() {
		return nil
	}
	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)
	orm.WithModel(ctx, &Team{})

	identities := append([]string{userID}, aliases...)
	const pageSize = 500
	var ids []string
	seen := map[string]struct{}{}
	for from := 0; ; from += pageSize {
		res, err := orm.SearchV2(ctx, orm.NewQuery().
			Should(orm.TermsQuery("members", identities)).
			MinimumShouldMatch(1).
			From(from).
			Size(pageSize))
		if err != nil {
			return nil
		}
		teams, total, err := elastic.DecodeHits[Team](res)
		if err != nil {
			return nil
		}
		for _, t := range teams {
			if _, dup := seen[t.ID]; dup {
				continue
			}
			for _, m := range t.Members {
				if memberMatches(m, userID, aliases) {
					seen[t.ID] = struct{}{}
					ids = append(ids, t.ID)
					break
				}
			}
		}
		if len(teams) < pageSize || int64(from+len(teams)) >= total {
			break
		}
	}
	return ids
}

// memberMatches reports whether one membership entry references the user:
// by user ID or any login alias.
func memberMatches(member, userID string, aliases []string) bool {
	if userID != "" && member == userID {
		return true
	}
	for _, a := range aliases {
		if a != "" && member == a {
			return true
		}
	}
	return false
}
