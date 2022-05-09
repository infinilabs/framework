/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elasticsearch

import (
	"fmt"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security/rbac"
	"infini.sh/framework/core/util"
	"strings"
)

type Role struct {
}

func (dal *Role) Get(id string) (rbac.Role, error){
	role := rbac.Role{
		ID: id,
	}
	_, err := orm.Get(&role)
	return role, err
}

func (dal *Role) Update(role *rbac.Role) error {
	return orm.Save(role)
}

func (dal *Role) Create(role *rbac.Role) (string, error){
	role.ID = util.GetUUID()
	return role.ID, orm.Save(role)
}

func (dal *Role) Delete(id string) error{
	role := rbac.Role{
		ID: id,
	}
	return orm.Delete(role)
}

func (dal *Role) Search(keyword string, from, size int) ( orm.Result, error){
	query := orm.Query{}

	queryDSL := `{"query":{"bool":{"must":[%s]}}, "from": %d,"size": %d}`
	mustBuilder := &strings.Builder{}

	if keyword != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"query_string":{"default_field":"*","query": "%s"}}`, keyword))
	}
	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), from, size)
	query.RawQuery = []byte(queryDSL)

	err, result := orm.Search(rbac.Role{}, &query)
	return result, err
}
