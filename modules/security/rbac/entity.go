/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package rbac

import (
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/entity_card"
)

type UserEntityProvider struct {
}

func (this *UserEntityProvider) GenEntityInfo(t string, id string) *entity_card.EntityInfo {
	_, u, _ := security.GetUserByID(id)
	if u != nil {
		//support user only now
		//get user's info via cloud API
		//output to card structure
		card := entity_card.EntityInfo{}
		card.Type = t
		card.ID = id
		card.Icon = "circle-user"
		card.Title = u.Name
		card.Subtitle = u.Email
		card.Cover = "https://blog.infinilabs.com/images/posts/2024/welcome-to-our-blog_hu8043560849171410142.jpg"
		return &card
	} else {
		return nil
	}
}

func (this *UserEntityProvider) GenEntityLabel(t string, ids []string) []entity_card.EntityLabel {
	output := []entity_card.EntityLabel{}

	builder := orm.NewQuery()
	builder.Must(orm.TermsQuery("id", ids))

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &security.UserAccount{})
	out := []security.UserAccount{}
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &out, builder, nil)
	if err != nil {
		panic(err)
	}

	for _, a := range out {
		l := entity_card.EntityLabel{}
		l.Type = t
		l.ID = a.ID
		l.Icon = "circle-user"
		l.Title = a.Name
		l.Subtitle = a.Email
		output = append(output, l)
	}
	return output
}
