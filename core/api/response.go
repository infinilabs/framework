// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

type Response struct {
	Total  int64       `json:"total,omitempty"`
	Hit    interface{} `json:"hit,omitempty"`
	Id     string      `json:"_id,omitempty"`
	Result string      `json:"result,omitempty"`
}
type FoundResp struct {
	Found  bool        `json:"found"`
	Id     string      `json:"_id,omitempty"`
	Source interface{} `json:"_source,omitempty"`
}

func CreateResponse(id string) Response {
	return Response{
		Id:     id,
		Result: "created",
	}
}
func UpdateResponse(id string) Response {
	return Response{
		Id:     id,
		Result: "updated",
	}
}
func DeleteResponse(id string) Response {
	return Response{
		Id:     id,
		Result: "deleted",
	}
}
func NotFoundResponse(id string) FoundResp {
	return FoundResp{
		Id:    id,
		Found: false,
	}
}
func FoundResponse(id string, data interface{}) FoundResp {
	return FoundResp{
		Id:     id,
		Found:  true,
		Source: data,
	}
}
