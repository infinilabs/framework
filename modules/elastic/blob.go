/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

type Blob struct {
	Content string `json:"content,omitempty" elastic_mapping:"content: { type: binary, doc_values:false }"`
}
