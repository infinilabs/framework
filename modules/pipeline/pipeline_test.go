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

package pipeline

import (
	"testing"
	"time"

	corepipeline "infini.sh/framework/core/pipeline"
)

func TestDeleteTaskWaitsForLoopRelease(t *testing.T) {
	module := &PipeModule{}
	ctx := corepipeline.AcquireContext(corepipeline.PipelineConfigV2{})

	module.contexts.Store("task-1", ctx)
	module.configs.Store("task-1", corepipeline.PipelineConfigV2{Name: "task-1"})
	module.pipelines.Store("task-1", struct{}{})

	released := make(chan struct{})
	go func() {
		for !ctx.IsCanceled() {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(50 * time.Millisecond)
		ctx.SetLoopReleased()
		close(released)
	}()

	start := time.Now()
	module.deleteTask("task-1")
	elapsed := time.Since(start)

	select {
	case <-released:
	default:
		t.Fatal("expected deleteTask to wait for loop release")
	}

	if elapsed < 50*time.Millisecond {
		t.Fatalf("expected deleteTask to wait for loop release, returned after %v", elapsed)
	}
	if _, ok := module.contexts.Load("task-1"); ok {
		t.Fatal("expected context to be deleted")
	}
	if _, ok := module.configs.Load("task-1"); ok {
		t.Fatal("expected config to be deleted")
	}
	if _, ok := module.pipelines.Load("task-1"); ok {
		t.Fatal("expected pipeline to be deleted")
	}
}
