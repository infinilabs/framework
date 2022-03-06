/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

import (
	"fmt"
	"infini.sh/framework/core/util"
	"testing"
)

func TestBuildSearchTermAggregations(t *testing.T){
	aggs := BuildSearchTermAggregations([]SearchAggParam{
		{Field: "name", TermsAggParams: util.MapStr{
			"size": 100,
		}},{
			Field: "labels.health_status",
			TermsAggParams: util.MapStr{
				"size": 10,
			},
		},
	})
	fmt.Println(aggs)
}

func TestBuildSearchTermFilter(t *testing.T){
	filter := BuildSearchTermFilter(map[string][]string{
		"version": {"5.6.8", "2.4.6"},
		"tags": {"v5", "infini"},
	})
	fmt.Println(filter)
}