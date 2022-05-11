/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package native

import (
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/util"
	"os"
	"path"
)

func loadJsonConfig() interface{} {
	pwd, _ := os.Getwd()

	bytes, err := util.FileGetContent(path.Join(pwd, "/config/permission.json"))
	if err != nil {
		log.Errorf("load permission file error: %v", err)
		return nil
	}
	apis := make(map[string][]string)
	err = json.Unmarshal(bytes, &apis)
	if err != nil {
		log.Error("json config unmarshal err " + err.Error())
		return nil
	}

	permissions := map[string]interface{}{
		"index_privileges": apis["indices"],
	}
	delete(apis, "indices")
	permissions["cluster_privileges"] = apis
	return permissions
}

func Init() {
	permissions := loadJsonConfig()
	rbac.RegisterPermission(rbac.Elasticsearch, permissions)
}
