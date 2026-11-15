/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package team provides the framework-generic security team: a principal
// that groups multiple users (matched by user ID or login name) so access
// can be granted to the group as a whole.
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

// List returns all teams (team counts are small; callers page client-side).
func List(ctx *orm.Context) ([]Team, error) {
	orm.WithModel(ctx, &Team{})
	qb := orm.NewQuery().Size(500)
	res, err := orm.SearchV2(ctx, qb)
	if err != nil {
		return nil, err
	}
	teams, _, err := elastic.DecodeHits[Team](res)
	return teams, err
}

// TeamIDsForUser returns the IDs of the teams whose membership lists the
// given user, matching either the user ID or the login name. Used at login to
// populate the session's teams_id. Runs on a background context: it must not
// fail logins, so any storage error simply yields no teams.
func TeamIDsForUser(userID, login string) []string {
	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)
	orm.WithModel(ctx, &Team{})

	qb := orm.NewQuery().Size(500)
	res, err := orm.SearchV2(ctx, qb)
	if err != nil {
		return nil
	}
	teams, _, err := elastic.DecodeHits[Team](res)
	if err != nil {
		return nil
	}

	var ids []string
	for _, t := range teams {
		for _, m := range t.Members {
			if m == userID || (login != "" && m == login) {
				ids = append(ids, t.ID)
				break
			}
		}
	}
	return ids
}
