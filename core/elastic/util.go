/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"infini.sh/framework/core/util"
	"strings"
)

func RemoveDotFromIndexName(indexName, dotReplaceTo string) string{
	if util.PrefixStr(indexName,"."){
		indexName=dotReplaceTo+ strings.TrimPrefix(indexName,".")
	}
	if util.SuffixStr(indexName,"."){
		indexName=strings.TrimSuffix(indexName,".")+dotReplaceTo
	}
	return indexName
}