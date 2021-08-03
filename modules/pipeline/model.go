/*
Copyright Medcl (m AT medcl.net)

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

package pipeline

import (
	"encoding/json"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
	"time"
)

const configBucket = "PipelineConfig"

func GetPipelineConfig(id string) (*pipeline.PipelineConfig, error) {
	if id == "" {
		return nil, errors.New("empty id")
	}
	b, err := kv.GetValue(configBucket, []byte(id))
	if err != nil {
		return nil, err
	}
	if len(b) > 0 {
		v := pipeline.PipelineConfig{}
		err = json.Unmarshal(b, &v)
		return &v, err
	}
	return nil, errors.Errorf("not found, %s", id)
}

func GetPipelineList(from, size int) (int64, []pipeline.PipelineConfig, error) {
	var configs []pipeline.PipelineConfig

	query := orm.Query{From: from, Size: size}

	err, r := orm.Search(pipeline.PipelineConfig{}, &configs, &query)
	if r.Result != nil && configs == nil || len(configs) == 0 {
		convertPipeline(r, &configs)
	}
	return r.Total, configs, err
}

func CreatePipelineConfig(cfg *pipeline.PipelineConfig) error {
	t := time.Now().UTC()
	cfg.ID = util.GetUUID()
	cfg.Created = t
	cfg.Updated = t
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	err = kv.AddValue(configBucket, []byte(cfg.ID), b)
	if err != nil {
		return err
	}
	return orm.Save(cfg)
}

func UpdatePipelineConfig(id string, cfg *pipeline.PipelineConfig) error {
	t := time.Now().UTC()
	cfg.ID = id
	cfg.Updated = t
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	err = kv.AddValue(configBucket, []byte(cfg.ID), b)
	if err != nil {
		return err
	}
	return orm.Update(cfg)
}

func DeletePipelineConfig(id string) error {
	err := kv.DeleteKey(configBucket, []byte(id))
	if err != nil {
		return err
	}
	o := pipeline.PipelineConfig{ID: id}
	return orm.Delete(&o)
}

func convertPipeline(result orm.Result, pipelines *[]pipeline.PipelineConfig) {
	if result.Result == nil {
		return
	}

	t, ok := result.Result.([]interface{})
	if ok {
		for _, i := range t {
			js := util.ToJson(i, false)
			t := pipeline.PipelineConfig{}
			util.FromJson(js, &t)
			*pipelines = append(*pipelines, t)
		}
	}
}
