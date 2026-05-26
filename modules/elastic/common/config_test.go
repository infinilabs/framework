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

package common

import (
	"testing"

	"infini.sh/framework/core/elastic"
)

func TestGetInitialMetadataHealthDefaultsToAvailableForNewCluster(t *testing.T) {
	if !getInitialMetadataHealth(nil) {
		t.Fatal("expected new cluster metadata to start as available before first health check")
	}
}

func TestGetInitialMetadataHealthKeepsExistingAvailability(t *testing.T) {
	meta := &elastic.ElasticsearchMetadata{Config: &elastic.ElasticsearchConfig{Enabled: true}}
	meta.Init(false)

	if getInitialMetadataHealth(meta) {
		t.Fatal("expected existing unavailable metadata to remain unavailable")
	}

	meta.Init(true)
	if !getInitialMetadataHealth(meta) {
		t.Fatal("expected existing available metadata to remain available")
	}
}
