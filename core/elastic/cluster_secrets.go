/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"errors"
	"strings"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/util"
	ucfg "infini.sh/framework/lib/go-ucfg"
	keystore2 "infini.sh/framework/lib/keystore"
)

// ──────────────────────────────────────────────────────────────────────────
// Cluster-scoped secrets in the keystore.
//
// ElasticsearchConfig credentials (basic_auth.password, token) are
// ucfg.SecretString fields: json.Marshal emits only the raw part, and a
// plain-text secret's raw part is the shadow text ("******"). A cluster
// saved through the ORM therefore persists the MASK, never the real
// credential - every consumer that later loads the record from the ORM
// (boot-time live registration, the cluster-change hook, app-side
// resolvers such as LogPilot's stream search) would authenticate with the
// mask and get 401s from secured clusters.
//
// The keystore is the durable home for the real values:
//
//   - StashClusterSecrets / StashClusterSecretsFromDelta run on cluster
//     create/update (the /easysearch/ CRUD), before the ORM write, and
//     store any plain-text credential under a cluster-scoped key.
//   - HydrateClusterSecrets runs wherever an ORM-loaded record is about to
//     be used to build a client or authenticate a request, replacing a
//     missing/masked secret with the keystore value.
//   - RemoveClusterSecrets drops the keys when the cluster is deleted.
//
// The ORM record keeps the mask, so API responses (which marshal the ORM
// object) never leak the real credential.
// ──────────────────────────────────────────────────────────────────────────

const clusterSecretKeyPrefix = "cluster_secret"

// ClusterBasicAuthPasswordKey is the keystore key holding a cluster's
// basic_auth password.
func ClusterBasicAuthPasswordKey(clusterID string) string {
	return clusterSecretKeyPrefix + "/" + clusterID + "/basic_auth_password"
}

// ClusterTokenKey is the keystore key holding a cluster's Easysearch API
// token.
func ClusterTokenKey(clusterID string) string {
	return clusterSecretKeyPrefix + "/" + clusterID + "/token"
}

// StashClusterSecrets saves real (non-masked) credentials carried by cfg
// into the keystore. Called on cluster create, before the ORM write.
// Masked or empty values are skipped: the API masks secrets in responses,
// so an edit that doesn't retype the password round-trips the mask and
// must not clear the stored secret.
func StashClusterSecrets(cfg *ElasticsearchConfig) {
	if cfg == nil || cfg.ID == "" {
		return
	}
	if cfg.BasicAuth != nil && cfg.BasicAuth.Username != "" {
		if pw := cfg.BasicAuth.Password.Get(); isRealSecret(pw) {
			stashClusterSecret(ClusterBasicAuthPasswordKey(cfg.ID), pw, ClusterTokenKey(cfg.ID))
		}
	}
	if tok := cfg.Token.Get(); isRealSecret(tok) {
		stashClusterSecret(ClusterTokenKey(cfg.ID), tok, ClusterBasicAuthPasswordKey(cfg.ID))
	}
}

// StashClusterSecretsFromDelta is StashClusterSecrets for the raw
// create/update request body (partial-update mode hands the hook a sparse
// object; the credentials only exist in the delta map). Switching auth
// mode (a new password vs a new token) drops the other key so hydration
// cannot resurrect a stale credential.
func StashClusterSecretsFromDelta(clusterID string, delta util.MapStr) {
	if clusterID == "" || len(delta) == 0 {
		return
	}
	if ba := asMapStr(delta["basic_auth"]); ba != nil {
		username, _ := ba["username"].(string)
		password, _ := ba["password"].(string)
		if username != "" && isRealSecret(password) {
			stashClusterSecret(ClusterBasicAuthPasswordKey(clusterID), password, ClusterTokenKey(clusterID))
		}
	}
	if tok, ok := delta["token"].(string); ok && isRealSecret(tok) {
		stashClusterSecret(ClusterTokenKey(clusterID), tok, ClusterBasicAuthPasswordKey(clusterID))
	}
}

// HydrateClusterSecrets fills masked or missing credentials on cfg with
// the real values from the keystore. Values already in memory (e.g. a
// request that just carried the plain-text secret) are kept. Call this
// wherever an ORM-loaded cluster record is about to be used to build a
// client, compare connection identity, or authenticate a request.
func HydrateClusterSecrets(cfg *ElasticsearchConfig) {
	if cfg == nil || cfg.ID == "" {
		return
	}
	if cfg.BasicAuth != nil && cfg.BasicAuth.Username != "" && !isRealSecret(cfg.BasicAuth.Password.Get()) {
		v, ok := loadClusterSecret(ClusterBasicAuthPasswordKey(cfg.ID))
		if ok {
			if v == "" {
				log.Warnf("cluster [%s]'s basic auth password is empty: %v", cfg.ID)
			}
			cfg.BasicAuth.Password = ucfg.SecretString(v)
		} else {
			log.Warnf("cluster [%s]'s basic auth password not found in the keystore: %v", cfg.ID)
		}
	}
	if !isRealSecret(cfg.Token.Get()) {
		v, ok := loadClusterSecret(ClusterTokenKey(cfg.ID))
		if ok {
			if v == "" {
				log.Warnf("cluster [%s]'s token is empty: %v", cfg.ID)
			}
			cfg.Token = ucfg.SecretString(v)
		} else {
			log.Warnf("cluster [%s]'s token not found in the keystore: %v", cfg.ID)
		}
	}
}

// RemoveClusterSecrets drops the cluster's keystore entries. Called when
// the cluster record is deleted.
func RemoveClusterSecrets(clusterID string) {
	if clusterID == "" {
		return
	}
	for _, key := range []string{ClusterBasicAuthPasswordKey(clusterID), ClusterTokenKey(clusterID)} {
		if err := keystore.DeleteValue(key); err != nil {
			log.Debugf("cluster %s: remove keystore secret %s: %v", clusterID, key, err)
		}
	}
}

// stashClusterSecret stores value under key and clears otherKey (the
// credential of the alternative auth mode), best-effort.
func stashClusterSecret(key, value, otherKey string) {
	if err := keystore.SetValue(key, []byte(value)); err != nil {
		log.Warnf("keystore: stash cluster secret %s failed: %v", key, err)
		return
	}
	if err := keystore.DeleteValue(otherKey); err != nil {
		log.Debugf("keystore: drop stale cluster secret %s: %v", otherKey, err)
	}
}

// Return value: (trimmed value, a bool indidcating if key eixsts)
func loadClusterSecret(key string) (string, bool) {
	v, err := keystore.GetValue(key)
	if err != nil {
		if !errors.Is(err, keystore2.ErrKeyDoesntExists) {
			log.Debugf("keystore: load cluster secret %s: %v", key, err)
		}
		return "", false
	}
	s := strings.TrimSpace(string(v))
	return s, true
}

// isRealSecret reports whether s carries an actual secret: not empty and
// not the marshal mask.
func isRealSecret(s string) bool {
	return s != "" && s != ucfg.SecretShadowText
}

func asMapStr(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case util.MapStr:
		return m
	}
	return nil
}
