/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package routetree

import (
	"encoding/json"
	"infini.sh/framework/core/util"
	"strings"
	"testing"
)

func TestTree(t *testing.T) {
	router := New()
	router.Handle("get", "/_cat/indices", "cat.indices")
	router.Handle("get", "/_cluster/health", "cluster.health")
	router.Handle("get", "/:index_name/_flush", "indices.flush")
	router.Handle("post", "/_snapshot/:repo_name/_verify", "snapshot.verify_repository")
	router.Handle("post", "/_snapshot/:repo_name/:snapshot_name", "snapshot.create")
	router.Handle("put", "/:index_name/:doctype/:doc_id", "doc.update")
	router.Handle("get", "/:index_name/_source/:doc_id", "doc.get")
	router.Handle("get", "/:index_name/:doctype/:doc_id", "doc.get")
	testRouter(t, router)
}

func testRouter(t *testing.T, router *Router){
	// /:index_name/:doctype/:doc_id
	path := "/test-update/_doc/1"
	path = path[1:]
	pathLen := len(path)
	trailingSlash := path[pathLen-1] == '/' && pathLen > 1
	redirectTrailingSlash := true
	if trailingSlash && redirectTrailingSlash {
		path = path[:pathLen-1]
	}
	permission, params, matched := router.Search("put", path)
	if !matched {
		t.Errorf("got matched equals %v, expect true", matched)
		return
	}
	if permission != "doc.update" {
		t.Errorf("got permission equals %v, expect doc.update", permission)
		return
	}
	if params == nil {
		t.Errorf("got empty params")
		return
	}
	if params["index_name"] != "test-update" {
		t.Errorf("got param index_name equals %v, expect test-update", params["index_name"])
	}
	if params["doctype"] != "_doc" {
		t.Errorf("got param doctype equals %v, expect _doc", params["doctype"])
	}
	if params["doc_id"] != "1" {
		t.Errorf("got param doc_id equals %v, expect 1", params["doc_id"])
	}
}

func TestTreeFromFile(t *testing.T) {
	router := load()
	testRouter(t, router)
}

type ElasticsearchAPIMetadata struct {
	Name string `json:"name"`
	Methods []string `json:"methods"`
	Path string `json:"path"`
}
type ElasticsearchAPIMetadataList []ElasticsearchAPIMetadata
func load() *Router{
	bytes, _ := util.FileGetContent("/Users/liugq/go/src/infini.sh/console/config/permission.json")

	apis := map[string]ElasticsearchAPIMetadataList{}
	json.Unmarshal(bytes, &apis)
	var esAPIRouter = New()
	for _, list := range apis {
		for _, md := range list {
			//skip wildcard *
			if strings.HasSuffix(md.Path, "*") {
				continue
			}
			for _, method := range md.Methods {
				esAPIRouter.Handle(method, md.Path, md.Name)
			}
		}
	}
	return esAPIRouter
}