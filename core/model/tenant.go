/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package model

import "infini.sh/framework/core/orm"

type TenantType string

const (
	Free    TenantType = "free"
	Paid    TenantType = "paid"
	Expired TenantType = "expired"
	Blocked TenantType = "blocked"
)

type Tenant struct {
	orm.ORMObjectBase
	Organization string     `json:"organization,omitempty"`
	Slogan       string     `json:"slogan,omitempty"`
	Website      string     `json:"website,omitempty"`
	Type         TenantType `json:"type,omitempty"`
	Industry     string     `json:"industry,omitempty"`
	Contact      string     `json:"contact,omitempty"`
	Phone        string     `json:"phone,omitempty"`
	Email        string     `json:"email,omitempty"`
	Address      string     `json:"address,omitempty"`
	Domain       string     `json:"domain,omitempty"` //customized domain
	TeamSize     string     `json:"team_size,omitempty"`
	BrandSetting struct {
		IconUrl          string `json:"icon_url,omitempty"`
		LogoUrl          string `json:"logo_url,omitempty"`
		BannerUrl        string `json:"banner_url,omitempty"`
		BackgroundUrl    string `json:"background_url,omitempty"`
		DefaultAvatarUrl string `json:"default_avatar_url,omitempty"`
	} `json:"brand_setting,omitempty"`
}

type TenantInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type TeamInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type ProjectInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}