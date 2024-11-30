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

package elastic

import (
	"github.com/segmentio/encoding/json"
	"sort"
	"strings"
)

func GetFieldCaps(client API, pattern string, metaFields []string) ([]ElasticField, error){
	buf, err := client.FieldCaps(pattern)
	if err != nil {
		return nil, err
	}
	var fieldCaps = &FieldCapsResponse{}
	err = json.Unmarshal(buf, fieldCaps)
	if err != nil {
		return nil, err
	}
	var esFields = []ElasticField{}
	for filedName, fieldCaps := range fieldCaps.Fields {
		if strings.HasPrefix(filedName, "_") && !isValidMetaField(filedName, metaFields){
			continue
		}
		var (
			typ string
			searchable bool
			aggregatable bool
			esTypes []string
			readFromDocValues bool
		)

		for esType, capsByType := range fieldCaps {
			if len(fieldCaps) > 1 {
				typ = "conflict"
			}else{
				typ = castEsToKbnFieldTypeName(esType)
			}
			esTypes = append(esTypes, esType)
			searchable = capsByType.Searchable
			aggregatable = capsByType.Aggregatable
			readFromDocValues = shouldReadFieldFromDocValues(esType, aggregatable)
		}
		if typ == "object" || typ == "nested"{
			continue
		}
		esFields = append(esFields, ElasticField{
			Name: filedName,
			Aggregatable:  aggregatable,
			Type: typ,
			Searchable: searchable,
			ReadFromDocValues: readFromDocValues,
			ESTypes: esTypes,
		})
	}
	sort.Slice(esFields, func(i, j int)bool{
		return esFields[i].Name < esFields[j].Name
	})
	return esFields, nil
}

func isValidMetaField(fieldName string, metaFields []string) bool {
	for _, f := range metaFields {
		if f == fieldName {
			return true
		}
	}
	return false
}

func shouldReadFieldFromDocValues(esType string, aggregatable bool) bool {
	return aggregatable && !(esType == "text" || esType == "geo_shape") && !strings.HasPrefix(esType, "_")
}

func castEsToKbnFieldTypeName(esType string) string {
	kbnTypes := createElasticFieldTypes()
	for _, ftype := range kbnTypes {
		for _, esType1 := range ftype.ESTypes {
			if esType1 == esType {
				return ftype.Name
			}
		}
	}
	return "unknown"
}


type ElasticField struct {
	Aggregatable bool `json:"aggregatable"`
	ESTypes []string `json:"esTypes"`
	Name string `json:"name"`
	ReadFromDocValues bool `json:"readFromDocValues"`
	Searchable bool `json:"searchable"`
	Type string `json:"type"`
}

type ElasticFieldType struct {
	Name string
	ESTypes []string
}
func createElasticFieldTypes() []ElasticFieldType {
	return []ElasticFieldType{
		{
			Name: "string",
			ESTypes: []string{
				"text", "keyword", "_type", "_id","_index","string",
			},
		},{
			Name:"number",
			ESTypes: []string{
				"float", "half_float", "scaled_float", "double","integer", "long", "unsigned_long", "short", "byte","token_count",
			},
		},{
			Name: "date",
			ESTypes: []string{
				"date", "date_nanos",
			},
		},{
			Name:"ip",
			ESTypes: []string{
				"ip",
			},
		}, {
			Name:"boolean",
			ESTypes: []string{
				"boolean",
			},
		},{
			Name:"object",
			ESTypes: []string{
				"object",
			},
		},{
			Name:"nested",
			ESTypes: []string{
				"nested",
			},
		},{
			Name:"geo_point",
			ESTypes: []string{
				"geo_point",
			},
		},{
			Name:"geo_shape",
			ESTypes: []string{
				"geo_shape",
			},
		},{
			Name:"attachment",
			ESTypes: []string{
				"attachment",
			},
		},{
			Name:"murmur3",
			ESTypes: []string{
				"murmur3",
			},
		},{
			Name:"_source",
			ESTypes: []string{
				"_source",
			},
		},{
			Name:"histogram",
			ESTypes: []string{
				"histogram",
			},
		},{
			Name:"conflict",
		},{
			Name:"unknown",
		},
	}
}
