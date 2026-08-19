/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"time"

	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/modules/security/access_token"
)

// ──────────────────────────────────────────────────────────────────────────
// Manager tokens — the framework's standard access-token manager backs the
// post-approval credential (replacing the earlier custom InstanceToken
// scheme). Approving an instance mints a real AccessToken via
// access_token.CreateAPIToken:
//
//   - stored/validated/revoked through the standard machinery
//     (KV fast-lookup + ORM record, the token management UI/API)
//   - carries instance binding (Data.instance_id) so a token can never
//     authenticate a different instance
//   - permissions attachable later for scoped manager capabilities
//
// The plaintext is returned exactly once (register/approve response) —
// the same one-time-display rule as before.
// ──────────────────────────────────────────────────────────────────────────

const managerTokenType = "managed_instance"

// mintManagerToken creates a framework-standard access token bound to the
// instance. Returns the plaintext.
func mintManagerToken(instanceID, instanceName string) (string, error) {
	user := &security.UserSessionInfo{
		Provider: "configs_server",
		Login:    instanceID,
	}
	user.SetUserID(instanceID)
	user.Set("instance_id", instanceID)
	if instanceName != "" {
		user.Set("instance_name", instanceName)
	}

	res, err := access_token.CreateAPIToken(user,
		"manager "+instanceName, "manager credential for instance "+instanceID,
		managerTokenType, -1, nil)
	if err != nil {
		return "", err
	}
	token, _ := res["access_token"].(string)
	return token, nil
}

// matchesManagerToken reports whether the presented token is a valid
// framework access token minted FOR THIS INSTANCE (manager binding).
func matchesManagerToken(_ *orm.Context, instanceID, presented string) bool {
	if presented == "" {
		return false
	}
	t, err := access_token.GetToken(presented)
	if err != nil || t == nil {
		return false
	}
	if t.Type != managerTokenType {
		return false
	}
	if t.ExpireIn > 0 && t.ExpireIn < time.Now().Unix() {
		return false
	}
	bound, _ := t.Data["instance_id"].(string)
	return bound == instanceID
}
