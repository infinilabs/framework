// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package adapter

import (
	"testing"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
)

func TestGetClusterUUIDFallsBackToConfigWhenMetadataMissing(t *testing.T) {
	cfg := elastic.ElasticsearchConfig{
		ORMObjectBase: orm.ORMObjectBase{ID: "test-cluster-uuid-fallback"},
		Name:          "test-cluster-uuid-fallback",
		ClusterUUID:   "cluster-uuid-fallback",
	}

	t.Cleanup(func() {
		elastic.RemoveInstance(cfg.ID)
	})

	elastic.UpdateConfig(cfg)

	clusterUUID, err := GetClusterUUID(cfg.ID)
	if err != nil {
		t.Fatalf("expected cluster uuid from config fallback, got error: %v", err)
	}
	if clusterUUID != cfg.ClusterUUID {
		t.Fatalf("expected cluster uuid %q, got %q", cfg.ClusterUUID, clusterUUID)
	}
}
