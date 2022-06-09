/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package routetree

import (
	"encoding/json"
	"fmt"
	"infini.sh/framework/core/util"
	"path"
	"strings"
	"testing"
)

func TestTree(t *testing.T) {
	//tree := &node{path: "/"}
	//addPath(tree, "get", "/_cat/indices", "cat.indices")
	//addPath(tree, "get", "/_cluster/health", "cluster.health")
	//addPath(tree, "get", "/:index_name/_flush", "indices.flush")
	//addPath(tree, "post", "/_snapshot/:repo_name/_verify", "snapshot.verify_repository")
	//addPath(tree, "post", "/_snapshot/:repo_name/:snapshot_name", "snapshot.create")
	router := load()
	path := "/_cat/indices"
	path = path[1:]
	pathLen := len(path)
	trailingSlash := path[pathLen-1] == '/' && pathLen > 1
	redirectTrailingSlash := true
	if trailingSlash && redirectTrailingSlash {
		path = path[:pathLen-1]
	}
	//rnode, permission, params := tree.search("get", path)
	//if rnode !=nil {
	//	fmt.Println(rnode.leafWildcardNames)
	//}
	//fmt.Println(permission, params)
	handler, _, _ := router.Search("get", path)
	fmt.Println(handler)
}

func addPath(tree *node, method, path string, permission string){

	n := tree.addPath(path[1:], nil, false)

	n.setPermission(method, permission, false)
}

type ElasticsearchAPIMetadata struct {
	Name string `json:"name"`
	Methods []string `json:"methods"`
	Path string `json:"path"`
}
type ElasticsearchAPIMetadataList []ElasticsearchAPIMetadata
func load() *Router{
	bytes, _ := util.FileGetContent(path.Join("/Users/liugq/go/src/infini.sh/console", "/config/permission.json"))

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