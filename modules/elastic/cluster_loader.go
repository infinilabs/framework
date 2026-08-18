/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	log "github.com/cihub/seelog"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/elastic/common"
)

// The cluster-management REST API (/easysearch/ CRUD) has moved to the
// dedicated, decoupled modules/easysearch package. This file keeps only the
// boot-time live-registration path, which still belongs here because it
// depends on the elasticsearch client factory (modules/elastic/common). It will
// move as part of the broader elasticsearch-module refactor.

// LoadClustersFromORM loads dynamic clusters from the ORM backend (sqlite or
// any non-elastic store) and registers a live client for each. Called from
// ElasticModule.Start when RemoteConfigEnabled is false — i.e. when there's no
// "system ES" to read the cluster index from. This is what makes clusters
// created via the /easysearch/ API (served by modules/easysearch) usable as
// live clients after (re)start.
func LoadClustersFromORM() {
	ctx := orm.NewContext().DirectAccess()
	orm.WithModel(ctx, &elastic.ElasticsearchConfig{})
	res, err := orm.SearchV2(ctx, orm.NewQuery().Size(1000))
	if err != nil {
		log.Warnf("load clusters from ORM: %v", err)
		return
	}
	clusters, _, err := elastic.DecodeHits[elastic.ElasticsearchConfig](res)
	if err != nil {
		log.Warnf("load clusters from ORM: decode: %v", err)
		return
	}
	for _, cfg := range clusters {
		if _, err := common.InitElasticInstance(cfg); err != nil {
			log.Warnf("cluster %s (%s): init failed: %v", cfg.ID, cfg.Name, err)
		}
	}
	log.Infof("loaded %d cluster(s) from ORM", len(clusters))
}
