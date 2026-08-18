/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/sqlite"
)

// TestClusterLifecycleHook proves ORM cluster records and the live-client
// registry stay in sync without a restart: create registers a client,
// labels-only updates keep it (the health loop's persist path must not
// re-initialize), connection changes rebuild it, delete removes it.
func TestClusterLifecycleHook(t *testing.T) {
	handler := &sqlite.SQLiteORM{Config: sqlite.SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(t.TempDir(), "hook.db"),
	}}
	require.NoError(t, handler.Open())
	t.Cleanup(func() { handler.Close() })
	require.NoError(t, handler.RegisterSchemaWithName(elastic.ElasticsearchConfig{}, "cluster"))

	orm.Register("sqlite", handler)
	registerClusterHook()

	newCfg := func(id, endpoint string) elastic.ElasticsearchConfig {
		cfg := elastic.ElasticsearchConfig{}
		cfg.ID = id
		cfg.Name = id
		cfg.Endpoint = endpoint
		cfg.Distribution = elastic.Elasticsearch
		cfg.Version = "8.0.0" // preset → offline adapter selection, no probe
		cfg.Enabled = true
		return cfg
	}

	t.Run("create registers live client", func(t *testing.T) {
		cfg := newCfg("hook-1", "http://hook-1:9200")
		require.NoError(t, orm.Create(orm.NewContext(), &cfg))
		assert.NotNil(t, elastic.GetClientNoPanic("hook-1"))
	})

	t.Run("labels-only update keeps the client", func(t *testing.T) {
		before := elastic.GetClientNoPanic("hook-1")

		obj := elastic.ElasticsearchConfig{}
		obj.ID = "hook-1"
		require.NoError(t, orm.UpdatePartialFields(orm.NewContext(), &obj, util.MapStr{
			"labels": util.MapStr{"health_status": "green"}, // health-loop persist shape
		}))

		after := elastic.GetClientNoPanic("hook-1")
		assert.NotNil(t, after)
		assert.Same(t, before, after, "labels-only change must not rebuild the client")
	})

	t.Run("connection change rebuilds the client", func(t *testing.T) {
		before := elastic.GetClientNoPanic("hook-1")

		obj := elastic.ElasticsearchConfig{}
		obj.ID = "hook-1"
		require.NoError(t, orm.UpdatePartialFields(orm.NewContext(), &obj, util.MapStr{
			"endpoint": "http://hook-1-changed:9200",
		}))

		after := elastic.GetClientNoPanic("hook-1")
		assert.NotNil(t, after)
		assert.NotSame(t, before, after, "endpoint change must rebuild the client")
		reg := elastic.GetConfigNoPanic("hook-1")
		require.NotNil(t, reg)
		assert.Equal(t, "http://hook-1-changed:9200", reg.Endpoint)
	})

	t.Run("delete removes the client", func(t *testing.T) {
		// Realistic path: load the full record first (the /easysearch/ API
		// deletes a loaded config), then delete.
		cfg := elastic.ElasticsearchConfig{}
		cfg.ID = "hook-1"
		exists, err := orm.GetV2(orm.NewContext(), &cfg)
		require.NoError(t, err)
		require.True(t, exists)
		require.NoError(t, orm.Delete(orm.NewContext(), &cfg))
		assert.Nil(t, elastic.GetClientNoPanic("hook-1"))
		assert.Nil(t, elastic.GetConfigNoPanic("hook-1"))
	})

	t.Run("delete with ID-only model must not panic", func(t *testing.T) {
		cfg := elastic.ElasticsearchConfig{}
		cfg.ID = "hook-2"
		c2 := newCfg("hook-2", "http://hook-2:9200")
		require.NoError(t, orm.Create(orm.NewContext(), &c2))
		bare := elastic.ElasticsearchConfig{}
		bare.ID = "hook-2"
		require.NoError(t, orm.Delete(orm.NewContext(), &bare))
		assert.Nil(t, elastic.GetClientNoPanic("hook-2"))
	})

	t.Run("non-cluster models pass through", func(t *testing.T) {
		type bystander struct {
			orm.ORMObjectBase
			Name string `json:"name,omitempty"`
		}
		require.NoError(t, handler.RegisterSchemaWithName(bystander{}, "bystanders"))
		b := bystander{Name: "x"}
		b.ID = "b-1"
		require.NoError(t, orm.Create(orm.NewContext(), &b)) // must not panic or register anything
		assert.Nil(t, elastic.GetClientNoPanic("b-1"))
	})
}
