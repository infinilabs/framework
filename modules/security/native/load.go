/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package native

import (
	_ "embed"
	"path"
	"strings"

	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/api/routetree"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type ElasticsearchAPIMetadata struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods"`
	Path    string   `json:"path"`
}
type ElasticsearchAPIMetadataList []ElasticsearchAPIMetadata

func (list ElasticsearchAPIMetadataList) GetNames() []string {
	var names []string
	for _, md := range list {
		if !util.StringInArray(names, md.Name) {
			names = append(names, md.Name)
		}
	}
	return names
}

//go:embed permission.json
var permissionFile []byte

func loadJsonConfig() map[string]ElasticsearchAPIMetadataList {
	externalConfig := path.Join(global.Env().GetConfigDir(), "permission.json")
	if util.FileExists(externalConfig) {
		log.Infof("loading permission file from %s", externalConfig)
		bytes, err := util.FileGetContent(externalConfig)
		if err != nil {
			log.Errorf("load permission file failed, use embedded config, err: %v", err)
		} else {
			permissionFile = bytes
		}
	}
	apis := map[string]ElasticsearchAPIMetadataList{}
	err := json.Unmarshal(permissionFile, &apis)
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
