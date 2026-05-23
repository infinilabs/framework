package orm

import "testing"

type handlerProbeORM struct{}

func (handlerProbeORM) Count(o interface{}, query interface{}) (int64, error) { return 0, nil }
func (handlerProbeORM) GroupBy(o interface{}, selectField, groupField string, haveQuery string, haveValue interface{}) (error, map[string]interface{}) {
	return nil, nil
}
func (handlerProbeORM) GetIndexName(o interface{}) string { return "" }
func (handlerProbeORM) GetWildcardIndexName(o interface{}) string {
	return ""
}
func (handlerProbeORM) GetBy(field string, value interface{}, o interface{}) (error, Result) {
	return nil, Result{}
}
func (handlerProbeORM) DeleteBy(o interface{}, query interface{}) error { return nil }
func (handlerProbeORM) UpdateBy(o interface{}, query interface{}) error { return nil }
func (handlerProbeORM) Search(o interface{}, q *Query) (error, Result)  { return nil, Result{} }
func (handlerProbeORM) SearchWithResultItemMapper(resultArrayRef interface{}, itemMapFunc func(source map[string]interface{}, targetRef interface{}) error, q *Query) (error, *SimpleResult) {
	return nil, &SimpleResult{}
}
func (handlerProbeORM) SearchV2(ctx *Context, qb *QueryBuilder) (*SearchResult, error) {
	return &SearchResult{}, nil
}
func (handlerProbeORM) DeleteByQuery(ctx *Context, qb *QueryBuilder) (*DeleteByQueryResponse, error) {
	return &DeleteByQueryResponse{}, nil
}
func (handlerProbeORM) RegisterSchemaWithName(t interface{}, customizedName string) error { return nil }
func (handlerProbeORM) Save(ctx *Context, o interface{}) error                            { return nil }
func (handlerProbeORM) Create(ctx *Context, o interface{}) error                          { return nil }
func (handlerProbeORM) Update(ctx *Context, o interface{}) error                          { return nil }
func (handlerProbeORM) Delete(ctx *Context, o interface{}) error                          { return nil }
func (handlerProbeORM) Get(ctx *Context, o interface{}) (bool, error)                     { return false, nil }

func TestHasHandlerReportsRegistrationState(t *testing.T) {
	original := handler
	handler = nil
	t.Cleanup(func() {
		handler = original
	})

	if HasHandler() {
		t.Fatal("expected HasHandler to be false when no ORM handler is registered")
	}

	handler = handlerProbeORM{}
	if !HasHandler() {
		t.Fatal("expected HasHandler to be true after an ORM handler is registered")
	}
}
