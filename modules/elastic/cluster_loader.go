/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"encoding/json"
	"reflect"

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

// parseClusterHits extracts the _source array from an orm.SearchResult payload
// (ES-shaped JSON: {"hits":{"hits":[{"_source":{...}}]}}). Returns nil if the
// payload can't be parsed.
func parseClusterHits(res interface{}) json.RawMessage {
	if res == nil {
		return nil
	}
	type searchResult struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	rv := reflect.ValueOf(res)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		rv = rv.Elem()
	}
	payloadField := rv.FieldByName("Payload")
	if !payloadField.IsValid() {
		return nil
	}
	var raw []byte
	switch v := payloadField.Interface().(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return nil
	}
	var sr searchResult
	if json.Unmarshal(raw, &sr) != nil {
		return nil
	}
	if len(sr.Hits.Hits) == 0 {
		return nil
	}
	out := make([]json.RawMessage, 0, len(sr.Hits.Hits))
	for _, hit := range sr.Hits.Hits {
		out = append(out, hit.Source)
	}
	b, _ := json.Marshal(out)
	return b
}

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
	var clusters []elastic.ElasticsearchConfig
	if hits := parseClusterHits(res); hits != nil {
		_ = json.Unmarshal(hits, &clusters)
	}
	for _, cfg := range clusters {
		if _, err := common.InitElasticInstance(cfg); err != nil {
			log.Warnf("cluster %s (%s): init failed: %v", cfg.ID, cfg.Name, err)
		}
	}
	log.Infof("loaded %d cluster(s) from ORM", len(clusters))
}
