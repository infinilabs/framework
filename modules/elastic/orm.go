package elastic

import (
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	api "infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
)

var ErrNotFound = errors.New("record not found")

type ElasticORM struct {
	Client elastic.API
	Config common.ORMConfig
}

func (handler ElasticORM) GetWildcardIndexName(o interface{}) string {
	name := handler.GetIndexName(o)
	return fmt.Sprintf("%v*", name)
}

func (handler ElasticORM) GetIndexName(o interface{}) string {
	indexName := getIndexName(o)

	if handler.Config.IndexPrefix == "" {
		return indexName
	}
	return fmt.Sprintf("%s%s", handler.Config.IndexPrefix, indexName)
}

func (handler ElasticORM) Get(o interface{}) (bool, error) {

	id := getIndexID(o)
	if id == "" {
		return false, errors.Errorf("id was not found in object: %v", o)
	}

	response, err := handler.Client.Get(handler.GetIndexName(o), "_doc", getIndexID(o))
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

func (handler ElasticORM) GetBy(field string, value interface{}, t interface{}) (error, api.Result) {

	query := api.Query{}
	query.Conds = api.And(api.Eq(field, value))
	return handler.Search(t, &query)
}

func (handler ElasticORM) Save(o interface{}) error {
	_, err := handler.Client.Index(handler.GetIndexName(o), "_doc", getIndexID(o), o, "")
	return err
}

func (handler ElasticORM) Update(o interface{}) error {
	return handler.Save(o)
}

func (handler ElasticORM) Delete(o interface{}) error {
	_, err := handler.Client.Delete(handler.GetIndexName(o), "_doc", getIndexID(o))
	return err
}

func (handler ElasticORM) DeleteBy(o interface{}, query interface{}) error {
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

func (handler ElasticORM) UpdateBy(o interface{}, query interface{}) error {
	var (
		queryBody []byte
		ok bool
	)
	if queryBody, ok =  query.([]byte); !ok {
		return errors.New("type of param query should be byte array")
	}
	_, err := handler.Client.UpdateByQuery(handler.GetIndexName(o), queryBody)
	return err
}

func (handler ElasticORM) Count(o interface{}) (int64, error) {
	countResponse, err := handler.Client.Count(nil, handler.GetIndexName(o), nil)
	if err != nil {
		return 0, err
	}
	return countResponse.Count, err
}

func getQuery(c1 *api.Cond) interface{} {

	switch c1.QueryType {
	case api.Match:
		q := elastic.MatchQuery{}
		q.Set(c1.Field, c1.Value)
		return q
	case api.Terms:
		q := elastic.TermsQuery{}
		q.Set(c1.Field, c1.Value.([]interface{}))
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
	}
	panic(errors.Errorf("invalid query: %s", c1))
}

func (handler ElasticORM) Search(t interface{}, q *api.Query) (error, api.Result) {

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
		searchResponse, err = handler.Client.QueryDSL(indexName, q.QueryArgs, q.RawQuery)
	} else {

		if q.Conds != nil && len(q.Conds) > 0 {
			request.Query = &elastic.Query{}

			boolQuery := elastic.BoolQuery{}

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

			request.Query.BoolQuery = &boolQuery

		}

		if q.Sort != nil && len(*q.Sort) > 0 {
			for _, i := range *q.Sort {
				request.AddSort(i.Field, string(i.SortType))
			}
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

func (handler ElasticORM) GroupBy(t interface{}, selectField, groupField string, haveQuery string, haveValue interface{}) (error, map[string]interface{}) {

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
