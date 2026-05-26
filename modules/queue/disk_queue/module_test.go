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

package queue

import (
	"testing"

	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
)

func TestNormalizeDiskQueueConfigAppliesRobustWriteDefaults(t *testing.T) {
	cfg := &DiskQueueConfig{}

	normalizeDiskQueueConfig(cfg)

	if cfg.WriteTimeoutInMS != defaultWriteTimeoutInMS {
		t.Fatalf("unexpected write timeout: %d", cfg.WriteTimeoutInMS)
	}
	if cfg.WriteChanBuffer != defaultWriteChanBuffer {
		t.Fatalf("unexpected write chan buffer: %d", cfg.WriteChanBuffer)
	}
}

func TestNormalizeDiskQueueConfigKeepsExplicitWriteSettings(t *testing.T) {
	cfg := &DiskQueueConfig{
		WriteTimeoutInMS: 45 * 1000,
		WriteChanBuffer:  64,
	}

	normalizeDiskQueueConfig(cfg)

	if cfg.WriteTimeoutInMS != 45*1000 {
		t.Fatalf("write timeout should be preserved, got %d", cfg.WriteTimeoutInMS)
	}
	if cfg.WriteChanBuffer != 64 {
		t.Fatalf("write chan buffer should be preserved, got %d", cfg.WriteChanBuffer)
	}
}

func TestSetupDefaultsDeleteAfterCompress(t *testing.T) {
	env1 := EmptyEnv()
	global.RegisterEnv(env1)

	module := DiskQueue{}
	module.Setup()

	if !module.cfg.Compress.DeleteAfterCompress {
		t.Fatalf("delete_after_compress should default to true")
	}
}
