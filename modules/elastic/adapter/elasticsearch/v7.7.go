/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elasticsearch

import (
	"errors"
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
)

type ESAPIV7_7 struct {
	ESAPIV7_3
}

func (c *ESAPIV7_7) GetIndices(pattern string) (*map[string]elastic.IndexInfo, error) {
	format := "%s/_cat/indices%s?h=health,status,index,uuid,pri,rep,docs.count,docs.deleted,store.size,pri.store.size,segments.count&format=json&expand_wildcards=all"
	if pattern != "" {
		pattern = "/" + pattern
	}
	url := fmt.Sprintf(format, c.GetEndpoint(), pattern)

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	data := []elastic.CatIndexResponse{}
	err = json.Unmarshal(resp.Body, &data)
	if err != nil {
		return nil, err
	}

	indexInfo := map[string]elastic.IndexInfo{}
	for _, v := range data {
		info := elastic.IndexInfo{}
		info.ID = v.Uuid
		info.Index = v.Index
		info.Status = v.Status
		info.Health = v.Health

		info.Shards, _ = util.ToInt(v.Pri)
		info.Replicas, _ = util.ToInt(v.Rep)
		info.DocsCount, _ = util.ToInt64(v.DocsCount)
		info.DocsDeleted, _ = util.ToInt64(v.DocsDeleted)
		info.SegmentsCount, _ = util.ToInt64(v.SegmentCount)

		info.StoreSize = v.StoreSize
		info.PriStoreSize = v.PriStoreSize

		indexInfo[v.Index] = info
	}

	return &indexInfo, nil
}
