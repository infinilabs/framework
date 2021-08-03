package elastic

import (
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	api "infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
)


type ElasticORM struct {
	Client elastic.API
	Config common.ORMConfig
}

func (handler ElasticORM) GetIndexName(o interface{}) string {
	indexName:=getIndexName(o)

	if handler.Config.IndexPrefix==""{
		return indexName
	}
	return fmt.Sprintf("%s%s",handler.Config.IndexPrefix,indexName)
}

func (handler ElasticORM) Get(o interface{}) error {

	response, err := handler.Client.Get(handler.GetIndexName(o),"", getIndexID(o))
	if err != nil {
		return err
	}

	//TODO improve performance
	str := util.ToJson(response.Source, false)
	return util.FromJson(str, o)
}

func (handler ElasticORM) GetBy(field string, value interface{}, t interface{}, to interface{}) (error, api.Result) {

	query := api.Query{}
	query.Conds = api.And(api.Eq(field, value))
	return handler.Search(t, to, &query)
}

func (handler ElasticORM) Save(o interface{}) error {
	_, err := handler.Client.Index(handler.GetIndexName(o), "_doc", getIndexID(o), o)
	return err
}

func (handler ElasticORM) Update(o interface{}) error {
	return handler.Save(o)
}

func (handler ElasticORM) Delete(o interface{}) error {
	_, err := handler.Client.Delete(handler.GetIndexName(o), "_doc", getIndexID(o))
	return err
}

func (handler ElasticORM) Count(o interface{}) (int64, error) {
	countResponse, err := handler.Client.Count(handler.GetIndexName(o))
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

func (handler ElasticORM) Search(t interface{}, to interface{}, q *api.Query) (error, api.Result) {

	var err error

	request := elastic.SearchRequest{}

	request.From = q.From
	request.Size = q.Size

	var searchResponse *elastic.SearchResponse
	result := api.Result{}

	if len(q.RawQuery) > 0 {
		searchResponse, err = handler.Client.SearchWithRawQueryDSL(handler.GetIndexName(t), q.RawQuery)
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

		searchResponse, err = handler.Client.Search(handler.GetIndexName(t), &request)

	}

	if err != nil {
		return err, result
	}

	var array []interface{}

	for _, doc := range searchResponse.Hits.Hits {
		array = append(array, doc.Source)
	}

	result.Result = array
	result.Total = searchResponse.GetTotal()

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
