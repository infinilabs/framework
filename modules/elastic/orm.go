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

package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	api "infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"infini.sh/framework/modules/elastic/orm"
	"net/http"
	"reflect"
	"strings"
)

var ErrNotFound = errors.New("record not found")

type ElasticORM struct {
	Client elastic.API
	Config common.ORMConfig
}

var templateInited bool

func InitTemplate(force bool) {
	if templateInited {
		return
	}

	if force || moduleConfig.ORMConfig.InitTemplate {
		client := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))

		//infini default template
		if !moduleConfig.ORMConfig.SkipInitDefaultTemplate {
			client.InitDefaultTemplate(moduleConfig.ORMConfig.TemplateName, moduleConfig.ORMConfig.IndexPrefix)
		}

		//index templates
		if moduleConfig.ORMConfig.IndexTemplates != nil && len(moduleConfig.ORMConfig.IndexTemplates) > 0 {
			for k, v := range moduleConfig.ORMConfig.IndexTemplates {
				var skip = false
				if !moduleConfig.ORMConfig.OverrideExistsTemplate {
					exists, err := client.TemplateExists(k)
					if err != nil {
						panic(err)
					}
					skip = exists
				}

				log.Trace(skip, ",", k, ",", v)

				if !skip {
					v, err := client.PutTemplate(k, []byte(v))
					if err != nil {
						if v != nil {
							log.Error(string(v))
						}
						panic(err)
					}
				}
			}
		}

		//search templates
		if moduleConfig.ORMConfig.SearchTemplates != nil && len(moduleConfig.ORMConfig.SearchTemplates) > 0 {
			for k, v := range moduleConfig.ORMConfig.SearchTemplates {
				var skip = false
				if !moduleConfig.ORMConfig.OverrideExistsTemplate {
					exists, err := client.ScriptExists(k)
					if err != nil {
						panic(err)
					}
					skip = exists
				}

				log.Trace(skip, ",", k, ",", v)

				if !skip {
					script := util.MapStr{}
					script["script"] = util.MapStr{
						"lang":   "mustache",
						"source": v,
					}
					v, err := client.PutScript(k, util.MustToJSONBytes(script))
					if err != nil {
						if v != nil {
							log.Error(string(v))
						}
						panic(err)
					}
				}
			}
		}
	}
	templateInited = true
}

func (handler *ElasticORM) GetWildcardIndexName(o interface{}) string {
	name := handler.GetIndexName(o)
	return fmt.Sprintf("%v*", name)
}

func (handler *ElasticORM) GetIndexName(o interface{}) string {
	indexName := getIndexName(o)
	if handler.Config.IndexPrefix == "" {
		return indexName
	}
	return fmt.Sprintf("%s%s", handler.Config.IndexPrefix, indexName)
}

func (handler *ElasticORM) Get(o interface{}) (bool, error) {

	id := getIndexID(o)
	if id == "" {
		return false, errors.Errorf("id was not found in object: %v", o)
	}

	response, err := handler.Client.Get(handler.GetIndexName(o), "", getIndexID(o))

	if global.Env().IsDebug && response != nil {
		log.Trace(string(response.RawResult.Body))
	}

	if err != nil {
		return false, err
	}
	if response.RawResult.StatusCode == http.StatusNotFound {
		return false, ErrNotFound
	}
	str, err := response.GetBytesByJsonPath("_source")
	if err != nil {
		return false, err
	}

	if str == nil {
		return false, nil
	}

	err = util.FromJSONBytes(str, o)
	return true, err
}

func (handler *ElasticORM) GetBy(field string, value interface{}, t interface{}) (error, api.Result) {
	query := api.Query{}
	query.Conds = api.And(api.Eq(field, value))
	return handler.Search(t, &query)
}

func (handler *ElasticORM) Save(ctx *api.Context, o interface{}) error {
	var refresh string
	if ctx != nil {
		refresh = ctx.Refresh
	}
	_, err := handler.Client.Index(handler.GetIndexName(o), "", getIndexID(o), o, refresh)
	return err
}

// update operation will merge the new data into the old data
func (handler *ElasticORM) Update(ctx *api.Context, o interface{}) error {
	var refresh string
	if ctx != nil {
		refresh = ctx.Refresh
	}
	//toUpdateObj := o
	//if ctx == nil || ctx.Context == nil || ctx.Value(api.ProtectedFilterKey) != false {
	//	toUpdateObj = api.FilterFieldsByProtected(o, false)
	//}
	_, err := handler.Client.Update(handler.GetIndexName(o), "", getIndexID(o), o, refresh)
	return err
}

func (handler *ElasticORM) Delete(ctx *api.Context, o interface{}) error {
	var refresh string
	if ctx != nil {
		refresh = ctx.Refresh
	}
	_, err := handler.Client.Delete(handler.GetIndexName(o), "", getIndexID(o), refresh)
	return err
}

func (handler *ElasticORM) DeleteBy(o interface{}, query interface{}) error {
	var (
		queryBody []byte
		ok        bool
	)
	if queryBody, ok = query.([]byte); !ok {
		return errors.New("type of param query should be byte array")
	}
	_, err := handler.Client.DeleteByQuery(handler.GetIndexName(o), queryBody)
	return err
}

func (handler *ElasticORM) UpdateBy(o interface{}, query interface{}) error {
	var (
		queryBody []byte
		ok        bool
	)
	if queryBody, ok = query.([]byte); !ok {
		return errors.New("type of param query should be byte array")
	}
	_, err := handler.Client.UpdateByQuery(handler.GetIndexName(o), queryBody)
	return err
}

func (handler *ElasticORM) Count(o interface{}, query interface{}) (int64, error) {
	var queryBody []byte

	if query != nil {
		var ok bool
		if queryBody, ok = query.([]byte); !ok {
			return 0, errors.New("type of param query should be byte array")
		}
	}
	countResponse, err := handler.Client.Count(nil, handler.GetIndexName(o), queryBody)
	if err != nil {
		return 0, err
	}
	return countResponse.Count, err
}

func getQuery(c1 *api.Cond) interface{} {

	switch c1.QueryType {
	case api.QueryStringType:
		q := elastic.NewQueryString(c1.Value.(string))
		q.Fields(c1.Field)
		return q
	case api.PrefixQueryType:
		q := elastic.PrefixQuery{}
		q.Set(c1.Field, c1.Value.(string))
		return q
	case api.Match:
		q := elastic.MatchQuery{}
		q.Set(c1.Field, c1.Value)
		return q
	case api.Terms:
		q := elastic.TermsQuery{}
		q.Set(c1.Field, c1.Value.([]interface{}))
		return q
	case api.StringTerms:
		q := elastic.TermsQuery{}
		q.SetStringArray(c1.Field, c1.Value.([]string))
		return q
	case api.RangeGt:
		q := elastic.RangeQuery{}
		q.Gt(c1.Field, c1.Value)
		return q
	case api.RangeGte:
		q := elastic.RangeQuery{}
		q.Gte(c1.Field, c1.Value)
		return q
	case api.RangeLt:
		q := elastic.RangeQuery{}
		q.Lt(c1.Field, c1.Value)
		return q
	case api.RangeLte:
		q := elastic.RangeQuery{}
		q.Lte(c1.Field, c1.Value)
		return q
		//TODO support BoolQuery
	}
	panic(errors.Errorf("invalid query: %s", c1))
}

func (handler *ElasticORM) ResolveIndexName(ctx *api.Context) string {

	if ctx != nil {
		if len(api.GetIndices(ctx)) > 0 {
			return strings.Join(api.GetIndices(ctx), ",")
		}

		pattern := api.GetIndexPattern(ctx)
		if pattern != "" {
			return pattern
		}

		model := api.GetModel(ctx)
		if model != nil {
			if api.IsWildcardIndex(ctx) {
				return handler.GetWildcardIndexName(model)
			}
			return handler.GetIndexName(model)
		}
	}

	panic(errors.Errorf("can't find index: %s", ctx))
}

func (handler *ElasticORM) SearchV2(ctx *api.Context, qb *api.QueryBuilder) (*api.SearchResult, error) {

	var err error
	var result *api.SearchResult = &api.SearchResult{}

	request := elastic.SearchRequest{}

	if qb != nil {
		request.From = qb.FromVal()
		request.Size = qb.SizeVal()
	}

	if collapseField := api.GetCollapseField(ctx); collapseField != "" {
		request.Collapse = &elastic.Collapse{Field: collapseField}
	}

	var searchResponse *elastic.SearchResponse

	var indexName = handler.ResolveIndexName(ctx)

	var queryArgs = api.GetQueryArgs(ctx)

	//TODO  add global filter, per user per tenant, per permission etc.

	if qb != nil {
		dsl := orm.ToDSL(qb)
		if dsl != nil {
			////parse query, remove unused parameters
			//query := elastic.SearchRequest{}
			//err = util.FromJSONBytes(q.RawQuery,&query)
			//if err == nil {
			//	q.RawQuery = util.MustToJSONBytes(query)
			//}else{
			//	log.Error(err)
			//}

			log.Info("FINAL INDEX: ", indexName, ", DSL: ", util.MustToJSON(dsl))

			dslBytes := util.MustToJSONBytes(dsl)
			searchResponse, err = handler.Client.QueryDSL(nil, indexName, queryArgs, dslBytes)
		}
	} else {
		//check if it is templated query
		if tq := api.GetTemplatedQuery(ctx); tq != nil {

			log.Info("FINAL INDEX: ", indexName, ", TEMPLATED: ", util.MustToJSON(tq))

			searchResponse, err = handler.Client.SearchByTemplate(indexName, tq.TemplateID, tq.Parameters)
		}
	}

	if searchResponse != nil && searchResponse.RawResult != nil {
		result.Status = searchResponse.RawResult.StatusCode
		result.Payload = searchResponse.RawResult.Body
		log.Info(searchResponse.RawResult.StatusCode, string(searchResponse.RawResult.Body))
	}

	result.Error = &err

	return result, err
}

func (handler *ElasticORM) Search(t interface{}, q *api.Query) (error, api.Result) {

	var err error

	request := elastic.SearchRequest{}

	request.From = q.From
	request.Size = q.Size

	if q.CollapseField != "" {
		request.Collapse = &elastic.Collapse{Field: q.CollapseField}
	}

	var searchResponse *elastic.SearchResponse
	result := api.Result{}

	var indexName = q.IndexName
	if indexName == "" {
		indexName = handler.GetIndexName(t)
		if q.WildcardIndex {
			indexName = handler.GetWildcardIndexName(t)
		}
	}

	if len(q.RawQuery) > 0 {

		//TODO
		////parse query, remove unused parameters
		//query := elastic.SearchRequest{}
		//err = util.FromJSONBytes(q.RawQuery,&query)
		//if err == nil {
		//	q.RawQuery = util.MustToJSONBytes(query)
		//}else{
		//	log.Error(err)
		//}

		searchResponse, err = handler.Client.QueryDSL(nil, indexName, q.QueryArgs, q.RawQuery)
	} else if q.TemplatedQuery != nil {
		searchResponse, err = handler.Client.SearchByTemplate(indexName, q.TemplatedQuery.TemplateID, q.TemplatedQuery.Parameters)
	} else {

		if q.Filter != nil || q.Conds != nil && len(q.Conds) > 0 {
			request.Query = &elastic.Query{}
			boolQuery := elastic.BoolQuery{}

			if len(q.Conds) > 0 {
				for _, c1 := range q.Conds {
					q := getQuery(c1)
					switch c1.BoolType {
					case api.Must:
						boolQuery.Must = append(boolQuery.Must, q)
						break
					case api.MustNot:
						boolQuery.MustNot = append(boolQuery.MustNot, q)
						break
					case api.Should:
						boolQuery.Should = append(boolQuery.Should, q)
						break
					}
				}
			}

			if q.Filter != nil {
				filter := getQuery(q.Filter)
				//temp fix for must_not filters
				if q.Filter.BoolType == api.MustNot {
					boolQuery.MustNot = append(boolQuery.MustNot, filter)
				} else {
					boolQuery.Filter = filter
				}
			}

			request.Query.BoolQuery = &boolQuery
		}

		if q.Sort != nil && len(*q.Sort) > 0 {
			for _, i := range *q.Sort {
				request.AddSort(i.Field, string(i.SortType))
			}
		}

		if global.Env().IsDebug {
			log.Info(util.MustToJSON(request))
		}

		searchResponse, err = handler.Client.Search(indexName, &request)
	}

	if err != nil {
		return err, result
	}

	var array []interface{}

	//TODO remove
	for _, doc := range searchResponse.Hits.Hits {
		if _, ok := doc.Source["id"]; !ok {
			doc.Source["id"] = doc.ID
		}
		array = append(array, doc.Source)
	}

	result.Result = array
	result.Raw = searchResponse.RawResult.Body
	result.Total = searchResponse.GetTotal() //TODO improve performance

	return err, result
}

func (handler *ElasticORM) SearchWithResultItemMapper(resultArray interface{}, itemMapFunc func(source map[string]interface{}, targetRef interface{}) error, q *api.Query) (error, *api.SimpleResult) {
	var err error

	if q == nil {
		panic("invalid query")
	}

	request := elastic.SearchRequest{
		From: q.From,
		Size: q.Size,
	}

	// Handle collapsing if a collapse field is provided
	if q.CollapseField != "" {
		request.Collapse = &elastic.Collapse{Field: q.CollapseField}
	}

	var searchResponse *elastic.SearchResponse

	// Validate that resultArray is a pointer to a slice
	arrayValue := reflect.ValueOf(resultArray)
	if arrayValue.Kind() != reflect.Ptr || arrayValue.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("resultArray must be a pointer to a slice"), nil
	}

	sliceValue := arrayValue.Elem()
	elementType := sliceValue.Type().Elem() // Get the type of elements in the slice

	// Resolve the index name based on the element type if not provided in query
	indexName := q.IndexName
	if indexName == "" {
		// Create a new instance of the element type to resolve index name dynamically
		tempInstance := reflect.New(elementType).Interface()
		indexName = handler.GetIndexName(tempInstance)
		if q.WildcardIndex {
			indexName = handler.GetWildcardIndexName(tempInstance)
		}
	}

	// Perform the query based on the provided conditions
	if len(q.RawQuery) > 0 {
		searchResponse, err = handler.Client.QueryDSL(nil, indexName, q.QueryArgs, q.RawQuery)
	} else if q.TemplatedQuery != nil {
		searchResponse, err = handler.Client.SearchByTemplate(indexName, q.TemplatedQuery.TemplateID, q.TemplatedQuery.Parameters)
	} else {

		request.Query = &elastic.Query{}
		boolQuery := elastic.BoolQuery{}

		if q.Conds != nil && len(q.Conds) > 0 {
			for _, cond := range q.Conds {
				query := getQuery(cond)
				switch cond.BoolType {
				case api.Must:
					boolQuery.Must = append(boolQuery.Must, query)
				case api.MustNot:
					boolQuery.MustNot = append(boolQuery.MustNot, query)
				case api.Should:
					boolQuery.Should = append(boolQuery.Should, query)
				}
			}
		}

		if q.Filter != nil {
			filter := getQuery(q.Filter)
			//temp fix for must_not filters
			if q.Filter.BoolType == api.MustNot {
				boolQuery.MustNot = append(boolQuery.MustNot, filter)
			} else {
				boolQuery.Filter = filter
			}
		}

		request.Query.BoolQuery = &boolQuery

		// Add sorting if specified
		if q.Sort != nil && len(*q.Sort) > 0 {
			for _, sort := range *q.Sort {
				request.AddSort(sort.Field, string(sort.SortType))
			}
		}

		// Perform the search
		searchResponse, err = handler.Client.Search(indexName, &request)
	}

	// Handle search errors
	if err != nil {
		return err, nil
	}

	// Populate the resultArray with typed data
	for _, doc := range searchResponse.Hits.Hits {
		// Create a new instance of the target element type
		elem := reflect.New(elementType).Elem()

		//make sure id exists and always be _id
		doc.Source["id"] = doc.ID

		if itemMapFunc != nil {

			source := doc.Source
			// Map the document source into the element
			if err := itemMapFunc(source, elem.Addr().Interface()); err != nil { // Ensure passing a pointer to itemMapFunc
				return fmt.Errorf("failed to map document to struct: %w", err), nil
			}
		}

		// Append the populated element to the result slice
		sliceValue.Set(reflect.Append(sliceValue, elem))
	}

	result := api.SimpleResult{}
	result.Total = searchResponse.GetTotal()
	result.Raw = util.MustToJSONBytes(searchResponse)

	return nil, &result
}

func (handler *ElasticORM) GroupBy(t interface{}, selectField, groupField string, haveQuery string, haveValue interface{}) (error, map[string]interface{}) {

	//agg := elastic.NewTermsAggregation().Field(selectField).Size(10)
	//
	//indexName := getIndexName(t)
	//
	//result, err := handler.Client.Search(indexName, selectField, agg)
	//if err != nil {
	//	log.Error(err)
	//}
	//
	//finalResult := map[string]interface{}{}
	//
	//ok,items:= result.Aggregations[]
	//if ok {
	//	for _, item := range items {
	//		k := fmt.Sprintf("%v", item.Key)
	//		finalResult[k] = item.DocCount
	//		log.Trace(item.Key, ":", item.DocCount)
	//	}
	//}
	//
	//return nil, finalResult
	return nil, nil
}
