/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package modules

import (
	"github.com/infinitbyte/framework/core/module"
	"github.com/infinitbyte/framework/modules/api"
	"github.com/infinitbyte/framework/modules/cluster"
	"github.com/infinitbyte/framework/modules/elastic"
	"github.com/infinitbyte/framework/modules/elastic/index"
	"github.com/infinitbyte/framework/modules/filter"
	"github.com/infinitbyte/framework/modules/pipeline"
	"github.com/infinitbyte/framework/modules/queue"
	"github.com/infinitbyte/framework/modules/stats"
	"github.com/infinitbyte/framework/modules/storage"
	"github.com/infinitbyte/framework/modules/ui"
)

// Register is where modules are registered
func Register() {
	module.Register(module.Database, elastic.ElasticModule{})
	module.Register(module.Storage, storage.StorageModule{})
	module.Register(module.Filter, filter.FilterModule{})
	module.Register(module.Stats, stats.SimpleStatsModule{})
	module.Register(module.Queue, queue.DiskQueue{})
	module.Register(module.Index, index.IndexModule{})
	module.Register(module.System, pipeline.PipeModule{})
	module.Register(module.System, cluster.ClusterModule{})
	module.Register(module.API, api.APIModule{})
	module.Register(module.UI, ui.UIModule{})
}
