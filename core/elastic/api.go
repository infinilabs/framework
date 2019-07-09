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

package elastic

import "bytes"

type API interface {
	Init()

	ClusterHealth() *ClusterHealth

	CreateIndex(name string, settings map[string]interface{}) error

	Index(indexName string, id interface{}, data interface{}) (*InsertResponse, error)

	Bulk(data *bytes.Buffer)

	Get(indexName, id string) (*GetResponse, error)
	Delete(indexName, id string) (*DeleteResponse, error)
	Count(indexName string) (*CountResponse, error)
	Search(indexName string, query *SearchRequest) (*SearchResponse, error)
	SearchWithRawQueryDSL(indexName string, queryDSL []byte) (*SearchResponse, error)

	GetIndexSettings(indexNames string) (*Indexes, error)
	UpdateIndexSettings(indexName string, settings map[string]interface{}) error

	IndexExists(indexName string) (bool, error)
	DeleteIndex(name string) error

	GetMapping(copyAllIndexes bool, indexNames string) (string, int, *Indexes, error)

	UpdateMapping(indexName string, mappings []byte) ([]byte, error)

	NewScroll(indexNames string, scrollTime string, docBufferCount int, query string, slicedId, maxSlicedCount int, fields string) (*ScrollResponse, error)
	NextScroll(scrollTime string, scrollId string) (*ScrollResponse, error)

	TemplateExists(templateName string) (bool, error)
	PutTemplate(templateName string, template []byte) ([]byte, error)

	Refresh(name string) (err error)
}
