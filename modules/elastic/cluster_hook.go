/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"sync"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/elastic/common"
)

// ──────────────────────────────────────────────────────────────────────────
// Cluster lifecycle hook.
//
// Clusters managed through the ORM (the /easysearch/ CRUD API served by
// modules/easysearch) used to become live clients only on the next boot,
// when ElasticModule.Start ran LoadClustersFromORM. This hook closes that
// gap: create/update/delete of an ElasticsearchConfig record is reflected
// in the live-client registry immediately.
//
// Loop guard: the health loop persists status back onto the same records
// (updateClusterHealthStatusViaORM → orm.Update → this hook). Those updates
// touch only non-connection fields, so re-initialization is skipped unless
// the connection identity (endpoints/credentials/version/distribution)
// actually changed — otherwise every health persist would re-probe the
// cluster.
//
// Note: logpilot-style consumers that resolve clusters straight from the
// ORM (ResolveClusterEndpoint) were already restart-free; this hook serves
// registry consumers (GetClient, the health loop itself, metadata).
// ──────────────────────────────────────────────────────────────────────────

// registerClusterHookOnce guards the hook registration; RegisterDataOperationPostHook
// would otherwise accumulate duplicates across module lifecycles.
var registerClusterHookOnce sync.Once

func registerClusterHook() {
	registerClusterHookOnce.Do(func() {
		orm.RegisterDataOperationPostHook(100, handleClusterChange, orm.OpCreate, orm.OpUpdate, orm.OpDelete)
	})
}

// handleClusterChange keeps the live-client registry in sync with ORM
// cluster records. Non-cluster models pass through untouched.
func handleClusterChange(ctx *orm.Context, op orm.Operation, o interface{}) (*orm.Context, interface{}, error) {
	cfg, ok := o.(*elastic.ElasticsearchConfig)
	if !ok || cfg == nil {
		return ctx, o, nil
	}

	switch op {
	case orm.OpCreate:
		if _, err := common.InitElasticInstance(*cfg); err != nil {
			log.Warnf("cluster %s (%s): live init after create failed: %v", cfg.ID, cfg.Name, err)
		} else {
			log.Debugf("cluster %s (%s): live client registered after create", cfg.ID, cfg.Name)
		}

	case orm.OpUpdate:
		prev := elastic.GetConfigNoPanic(cfg.ID)
		switch {
		case prev == nil:
			// Not registered yet (e.g. record created while this hook was
			// absent) — bring it live now.
			if _, err := common.InitElasticInstance(*cfg); err != nil {
				log.Warnf("cluster %s (%s): live init after update failed: %v", cfg.ID, cfg.Name, err)
			}
		case elastic.SameConnectionIdentity(*prev, *cfg):
			// Labels/health-status-only change: keep the live client, but
			// refresh the registered config so the registry is not stale.
			if client := elastic.GetClientNoPanic(cfg.ID); client != nil {
				elastic.RegisterInstance(*cfg, client)
			}
		default:
			// Connection identity changed (endpoint/credentials/version):
			// drop the cached self-contained client and re-register fresh.
			elastic.InvalidateClient(*prev)
			if _, err := common.InitElasticInstance(*cfg); err != nil {
				log.Warnf("cluster %s (%s): live re-init after update failed: %v", cfg.ID, cfg.Name, err)
			} else {
				log.Debugf("cluster %s (%s): live client re-registered after connection change", cfg.ID, cfg.Name)
			}
		}

	case orm.OpDelete:
		elastic.RemoveInstance(cfg.ID)
		elastic.InvalidateClient(*cfg)
		log.Debugf("cluster %s (%s): live client removed after delete", cfg.ID, cfg.Name)
	}

	return ctx, o, nil
}
