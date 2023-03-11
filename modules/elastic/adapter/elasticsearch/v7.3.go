/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elasticsearch

import (
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
)

type ESAPIV7_3 struct {
	ESAPIV7
}

func (c *ESAPIV7_3) GetClusterState() (*elastic.ClusterState, error) {
	format := "%s/_cluster/state/version,master_node,routing_table,metadata/*?expand_wildcards=all"
	url := fmt.Sprintf(format, c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	obj := &elastic.ClusterState{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj, err
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode = resp.StatusCode
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj, err
	}
	return obj, nil
}

func (c *ESAPIV7_3) GetStats() (*elastic.Stats, error) {
	format := "%s/_stats?ignore_unavailable=true&expand_wildcards=all"
	url := fmt.Sprintf(format, c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	var response = &elastic.Stats{}
	err = json.Unmarshal(resp.Body, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}