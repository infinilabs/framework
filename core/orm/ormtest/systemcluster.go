/* Copyright © INFINI LTD. All rights reserved. */

package ormtest

import (
	"errors"
	"os"
	"sync"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	ucfg "infini.sh/framework/lib/go-ucfg"
)

var (
	seedOnce sync.Once
	seedErr  error
)

// ErrNoEndpoint is returned when ES_ENDPOINT is not set; callers skip.
var ErrNoEndpoint = errors.New("ES_ENDPOINT is not set")

// SeedSystemCluster saves the global-system cluster record built from ES_*
// environment variables so backend resolvers (elastic common.GetElasticClient)
// can find a live cluster in integration tests. The caller registers its own
// ORM handler first (each package wires the backend it ships with):
//
//	orm.Register("sqlite-integration", handler)
//	ormtest.SeedSystemCluster()
//
// Idempotent per process; both the elastic and sqlite test binaries call it.
func SeedSystemCluster() error {
	if os.Getenv("ES_ENDPOINT") == "" {
		return ErrNoEndpoint
	}
	seedOnce.Do(func() {
		cfg := elastic.ElasticsearchConfig{}
		cfg.ID = elastic.GlobalSystemElasticsearchID
		cfg.Name = "system"
		cfg.Endpoint = os.Getenv("ES_ENDPOINT")
		cfg.Enabled = true
		cfg.BasicAuth = &model.BasicAuth{
			Username: envOr("ES_USERNAME", "admin"),
			Password: ucfg.SecretString(envOr("ES_PASSWORD", "admin")),
		}
		seedErr = orm.Save(orm.NewContext(), &cfg)
	})
	return seedErr
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
