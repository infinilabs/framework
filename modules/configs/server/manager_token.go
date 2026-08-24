/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"time"

	log "github.com/cihub/seelog"

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
// maxManagerTokensPerInstance 每实例保留的 manager token 上限 (轮换容错:
// 旧 token 在新 token 送达实例前仍需可用; 再旧的属泄漏, 铸造时修剪)。
const maxManagerTokensPerInstance = 2

// pruneManagerTokens 删除同实例超出上限的旧 manager token。
func pruneManagerTokens(instanceID string) {
	tokens, err := access_token.ListTokens()
	if err != nil {
		return
	}
	type entry struct {
		id  string
		seq string
	}
	var mine []entry
	for i := range tokens {
		t := tokens[i] // 指针遍历: AccessToken 内含锁, 值拷贝被 vet 捕获
		if t.Type != managerTokenType {
			continue
		}
		if bound, _ := t.Data["instance_id"].(string); bound != instanceID {
			continue
		}
		mine = append(mine, entry{id: t.ID, seq: t.ID})
	}
	if len(mine) <= maxManagerTokensPerInstance {
		return
	}
	// id 是时序生成的 (k-sortid), 字典序即时间序: 保留最后 N 个。
	for i := 0; i < len(mine)-maxManagerTokensPerInstance; i++ {
		if err := access_token.DeleteTokenByID(mine[i].id); err == nil {
			log.Infof("configs server: pruned stale manager token %s for instance %s", mine[i].id, instanceID)
		}
	}
}

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
	if token != "" {
		pruneManagerTokens(instanceID)
	}
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

// MintPublicManagerToken is mintManagerToken for admin surfaces (rotation).
func MintPublicManagerToken(instanceID, instanceName string) (string, error) {
	return mintManagerToken(instanceID, instanceName)
}

// EnrollmentRequired reports whether the admission ticket gate is on.
func EnrollmentRequired() bool { return serverConfig.Enrollment.Required }

// StaticTokens returns the configured static gate tokens (empty = open mode).
func StaticTokens() []string { return staticTokens }
