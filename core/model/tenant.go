/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package model

import "infini.sh/framework/core/orm"

type Tenant struct {
	orm.ORMObjectBase
	Type         string `json:"type"`
	Industry     string `json:"industry"`
	BrandSetting struct {
		IconUrl          string `json:"icon_url"`
		BannerUrl        string `json:"banner_url"`
		BackgroundUrl    string `json:"background_url"`
		DefaultAvatarUrl string `json:"default_avatar_url"`
	} `json:"brand_setting"`
	Contact      string `json:"contact"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Organization string `json:"organization"`
	Address      string `json:"address"`
}


type TenantInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
