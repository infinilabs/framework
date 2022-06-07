/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package native

import (
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/api/routetree"
	"infini.sh/framework/core/util"
	"os"
	"path"
	"strings"
)

type ElasticsearchAPIMetadata struct {
	Name string `json:"name"`
	Methods []string `json:"methods"`
	Path string `json:"path"`
}
type ElasticsearchAPIMetadataList []ElasticsearchAPIMetadata
func (list ElasticsearchAPIMetadataList) GetNames() []string{
	var names []string
	for _, md := range list {
		if !util.StringInArray(names, md.Name){
			names = append(names, md.Name)
		}
	}
	return names
}

func loadJsonConfig() map[string]ElasticsearchAPIMetadataList {
	pwd, _ := os.Getwd()

	bytes, err := util.FileGetContent(path.Join(pwd, "/config/permission.json"))
	if err != nil {
		log.Errorf("load permission file error: %v", err)
		return nil
	}
	apis := map[string]ElasticsearchAPIMetadataList{}
	err = json.Unmarshal(bytes, &apis)
	if err != nil {
		log.Error("json config unmarshal err " + err.Error())
		return nil
	}

	return apis
}

func Init() {
	apis := loadJsonConfig()
	if apis != nil {
		var esAPIRouter = routetree.New()
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
		rbac.RegisterAPIPermissionRouter("elasticsearch", esAPIRouter)
	}

	permissions := map[string]interface{}{
		"index_privileges": apis["indices"].GetNames(),
	}
	delete(apis, "indices")
	otherApis := map[string][]string{}
	for key, list := range apis {
		otherApis[key] = list.GetNames()

	}
	permissions["cluster_privileges"] = otherApis
	rbac.RegisterPermission(rbac.Elasticsearch, permissions)
}
