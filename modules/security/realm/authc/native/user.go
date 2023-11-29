/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package native

import (
	"fmt"
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"strings"
)

type User struct {
}

func (dal *User) Get(id string) (*rbac.User, error) {
	user := rbac.User{}
	user.ID = id
	ok, err := orm.Get(&user)
	if ok && err == nil {
		//nickname always has value
		if user.Nickname==""&&user.Username!=""{
			user.Nickname=user.Username
		}
		return &user, err
	}
	return nil, errors.Error("failed to get user:%v, %v", id, err)
}

func (dal *User) GetBy(field string, value interface{}) (*rbac.User, error) {
	user := &rbac.User{
	}
	err, result := orm.GetBy(field, value, rbac.User{})
	if err != nil {
		return nil, err
	}
	if len(result.Result) == 0 {
		return nil, nil
	}
	userBytes, err := util.ToJSONBytes(result.Result[0])
	if err != nil {
		return nil, err
	}
	util.FromJSONBytes(userBytes, &user)
	return user, err
}

func (dal *User) Update(user *rbac.User) error {

	return orm.Update(nil, user)
}

func (dal *User) Create(user *rbac.User) (string, error) {
	if user.ID==""{
		user.ID = util.GetUUID()
	}
	return user.ID, orm.Save(nil, user)
}

func (dal *User) Delete(id string) error {
	user := rbac.User{}
	user.ID = id
	return orm.Delete(nil, user)
}

func (dal *User) Search(keyword string, from, size int) (orm.Result, error) {
	query := orm.Query{}

	queryDSL := `{"query":{"bool":{"must":[%s]}}, "from": %d,"size": %d}`
	mustBuilder := &strings.Builder{}

	if keyword != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"query_string":{"default_field":"*","query": "%s"}}`, keyword))
	}
	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), from, size)
	query.RawQuery = []byte(queryDSL)

	err, result := orm.Search(rbac.User{}, &query)
	return result, err
}
