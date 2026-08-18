/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/aggregate"
	api "infini.sh/framework/core/orm"
	sqliteOrm "infini.sh/framework/modules/sqlite/orm"
)

// ──────────────────────────────────────────────────────────────────────────
// Typed aggregation execution.
//
// Aggregate computes bucket/metric aggregations natively in SQL and hands
// pipeline aggregations to the framework engine (core/aggregate), so sqlite
// and elastic behave identically.
//
// Nesting strategy (design doc §6.2): ONE level query per bucket-aggregation
// node, grouped by (ancestor keys..., own key). Rows are partitioned by the
// ancestor tuple and linked back into the parent's buckets, so the query
// count scales with the spec-tree depth — not with the number of buckets
// (the P2 implementation issued one query per bucket value).
// percentiles and top_hits are per-bucket post passes (rare, documented).
// ──────────────────────────────────────────────────────────────────────────

// Aggregate implements orm.MetricsAPI.
func (handler *SQLiteORM) Aggregate(ctx *api.Context, qb *api.QueryBuilder) (*api.AggregationResult, error) {
	if qb == nil || len(qb.Aggs) == 0 {
		return nil, fmt.Errorf("no aggregations set on the query builder")
	}
	indexName, err := handler.resolveAggregateIndex(ctx)
	if err != nil {
		return nil, err
	}
	qb.Build()
	resolver := lookupTableSchema(indexName).resolver()
	where, args := sqliteOrm.BuildWhereClause(qb, resolver)

	exec := &aggExecutor{handler: handler, index: indexName, resolve: resolver, schema: lookupTableSchema(indexName), where: where, args: args}
	nodes, err := exec.execute(qb.Aggs)
	if err != nil {
		return nil, err
	}
	result := &api.AggregationResult{Aggs: nodes}
	if err := aggregate.ApplyPipelines(result, qb.Aggs); err != nil {
		return nil, err
	}
	return result, nil
}

// epochExpr returns the integer-epoch shadow expression for a date field
// when the registered schema has one.
func (e *aggExecutor) epochExpr(field string) string {
	if e.schema == nil {
		return ""
	}
	return e.schema.dateEpochByPath[field]
}

// resolveAggregateIndex resolves the target table like SearchV2.
func (handler *SQLiteORM) resolveAggregateIndex(ctx *api.Context) (string, error) {
	var indexName string
	if ctx != nil {
		if indices := api.GetIndices(ctx); len(indices) > 0 {
			indexName = indices[0]
		}
		if indexName == "" {
			if pattern := api.GetIndexPattern(ctx); pattern != "" {
				indexName = pattern
			}
		}
		if indexName == "" {
			if model := api.GetModel(ctx); model != nil {
				indexName = handler.GetIndexName(model)
			}
		}
	}
	if indexName == "" {
		return "", fmt.Errorf("cannot resolve table name from context")
	}
	return indexName, nil
}

// ──────────────────────────────────────────────────────────────────────────
// Executor
// ──────────────────────────────────────────────────────────────────────────

type aggExecutor struct {
	handler *SQLiteORM
	index   string
	resolve sqliteOrm.FieldResolver
	schema  *tableSchema // nil-safe: resolver falls back to json_extract
	where   string
	args    []interface{}
}

// bucketRow is one row of a level query.
type bucketRow struct {
	ancestors []interface{} // values of the ancestor group keys
	key       interface{}   // this level's group key
	docCount  int64
	metrics   []float64 // parallel to the level's metric specs
}

// namedXxx pair an aggregation name with its spec for deterministic order.
type namedMetric struct {
	name string
	agg  *api.MetricAggregation
}
type namedAgg struct {
	name string
	agg  api.Aggregation
}

type scopeChildren struct {
	metrics []namedMetric // computed in the level query
	buckets []namedAgg    // recursive level queries
	post    []namedAgg    // percentiles / top_hits: per-bucket post passes
}

// splitChildren separates a bucket agg's nested scope by execution strategy.
func splitChildren(spec map[string]api.Aggregation) scopeChildren {
	var c scopeChildren
	for name, sub := range spec {
		switch sub.(type) {
		case *api.MetricAggregation:
			c.metrics = append(c.metrics, namedMetric{name, sub.(*api.MetricAggregation)})
		case *api.TermsAggregation, *api.DateHistogramAggregation, *api.AutoDateHistogramAggregation:
			c.buckets = append(c.buckets, namedAgg{name, sub})
		case *api.PercentilesAggregation, *api.TopHitsAggregation:
			c.post = append(c.post, namedAgg{name, sub})
		}
	}
	sort.Slice(c.metrics, func(i, j int) bool { return c.metrics[i].name < c.metrics[j].name })
	sort.Slice(c.buckets, func(i, j int) bool { return c.buckets[i].name < c.buckets[j].name })
	sort.Slice(c.post, func(i, j int) bool { return c.post[i].name < c.post[j].name })
	return c
}

// execute runs a top-level scope.
func (e *aggExecutor) execute(spec map[string]api.Aggregation) (map[string]*api.AggNode, error) {
	out := map[string]*api.AggNode{}
	for name, agg := range spec {
		node, err := e.execOne(name, agg, nil, nil)
		if err != nil {
			return nil, err
		}
		if node == nil {
			// Empty result sets leave the partition map unpopulated; callers
			// expect a usable node (empty buckets), never nil.
			node = &api.AggNode{Buckets: []api.Bucket{}}
		}
		out[name] = node
	}
	return out, nil
}

// execOne executes one named aggregation. ancestors are the ancestor group
// expressions; scope pins ancestor VALUES (parent tuple) — either ancestors
// (level query) or scope (post pass), not both.
func (e *aggExecutor) execOne(name string, agg api.Aggregation, ancestors []string, scope *extraScope) (*api.AggNode, error) {
	switch a := agg.(type) {
	case *api.TermsAggregation:
		parts, err := e.execBucketPartitioned(a, ancestors, termsKeyPlan{field: a.Field, size: a.Size})
		if err != nil {
			return nil, err
		}
		return parts[""], nil // may be nil on an empty set; execute() nil-guards

	case *api.DateHistogramAggregation:
		interval := a.Interval
		if interval == "" && a.IntervalField != "" {
			interval = a.IntervalField
		}
		format, ok := histogramFormatFor(interval)
		if !ok {
			warnAggUnsupported("date_histogram with interval " + interval)
			return &api.AggNode{Buckets: []api.Bucket{}}, nil
		}
		parts, err := e.execBucketPartitioned(a, ancestors, dateKeyPlan{field: a.Field, format: format, interval: interval, offset: a.Offset, epochExpr: e.epochExpr(a.Field)})
		if err != nil {
			return nil, err
		}
		return parts[""], nil

	case *api.AutoDateHistogramAggregation:
		dh, err := e.autoIntervalDH(a)
		if err != nil {
			return nil, err
		}
		return e.execOne(name, dh, ancestors, scope)

	case *api.MetricAggregation:
		v, err := e.topMetric(a)
		if err != nil {
			return nil, err
		}
		node := &api.AggNode{}
		if v != nil {
			node.Value = *v
			node.ValueSet = true
		}
		return node, nil

	case *api.PercentilesAggregation:
		vals, err := e.percentiles(a, scope)
		if err != nil {
			return nil, err
		}
		return &api.AggNode{Values: vals}, nil

	case *api.TopHitsAggregation:
		return e.topHits(a, scope)

	case *api.DateRangeAggregation:
		return e.dateRange(a)

	case *api.FilterAggregation:
		return e.filterAgg(a)

	case *api.SamplerAggregation:
		warnAggUnsupported("sampler (falling back to full data)")
		return e.singleBucketScope(a.GetNested())

	default:
		// Pipelines get placeholder nodes; the framework engine fills them.
		if isPipelineAgg(agg) {
			return &api.AggNode{}, nil
		}
		warnAggUnsupported(fmt.Sprintf("%T", agg))
		return &api.AggNode{}, nil
	}
}

func isPipelineAgg(agg api.Aggregation) bool {
	switch agg.(type) {
	case *api.PipelineAggregation, *api.DerivativeAggregation,
		*api.BucketScriptAggregation, *api.BucketSortAggregation, *api.MaxBucketAggregation:
		return true
	}
	return false
}

// ──────────────────────────────────────────────────────────────────────────
// Bucket level machinery (one query per bucket-agg node)
// ──────────────────────────────────────────────────────────────────────────

// keyPlan produces the group-key expression and bucket identity for a level.
type keyPlan interface {
	keyExpr(resolve sqliteOrm.FieldResolver) string
	// bucketIdentity renders the row key into the typed Bucket fields.
	bucketIdentity(key interface{}) (keyStr string, keyRaw interface{})
	// order/fill behavior
	postProcess(buckets []api.Bucket) []api.Bucket
}

type termsKeyPlan struct {
	field string
	size  int
}

func (p termsKeyPlan) keyExpr(r sqliteOrm.FieldResolver) string { return sqliteOrm.ExprFor(r, p.field) }
func (p termsKeyPlan) bucketIdentity(key interface{}) (string, interface{}) {
	return fmt.Sprintf("%v", key), key
}

// postProcess applies ES terms semantics: order by doc_count desc with
// key-ascending tie-break, then truncate to size.
func (p termsKeyPlan) postProcess(buckets []api.Bucket) []api.Bucket {
	sort.SliceStable(buckets, func(i, j int) bool {
		if buckets[i].DocCount != buckets[j].DocCount {
			return buckets[i].DocCount > buckets[j].DocCount
		}
		return buckets[i].Key < buckets[j].Key
	})
	if p.size > 0 && len(buckets) > p.size {
		buckets = buckets[:p.size]
	}
	return buckets
}

type dateKeyPlan struct {
	field     string
	format    string
	interval  string
	offset    time.Duration // ES date_histogram offset: bucket = floor((t-offset)/interval)
	step      time.Duration // resolved interval step (0 → derive from interval string)
	epochExpr string        // integer-epoch shadow column expression ("" → strftime fallback)
}

// keyExpr buckets by integer division of the epoch seconds:
//
//	(CAST(strftime('%%s', expr) AS INTEGER) - offsetSecs) / intervalSecs
//
// ONE strftime per row (the nested-modifier variant cost two string parses
// per row and only supported fixed formats); epoch division also handles
// arbitrary interval lengths (6h, 90m, ...). The returned key is the bucket
// INDEX; bucketIdentity renders it back to start-epoch/key-string.
func (p dateKeyPlan) keyExpr(r sqliteOrm.FieldResolver) string {
	step := p.intervalStep()
	if p.epochExpr != "" {
		// Integer arithmetic on the materialized epoch shadow — no per-row
		// RFC3339 parse.
		return fmt.Sprintf("((%s - %d) / %d)", p.epochExpr, int(p.offset.Seconds()), int(step.Seconds()))
	}
	expr := sqliteOrm.ExprFor(r, p.field)
	return fmt.Sprintf("((CAST(strftime('%%s', %s) AS INTEGER) - %d) / %d)",
		expr, int(p.offset.Seconds()), int(step.Seconds()))
}

func (p dateKeyPlan) intervalStep() time.Duration {
	if p.step > 0 {
		return p.step
	}
	switch strings.TrimSpace(p.interval) {
	case "1m", "1minute", "minute":
		return time.Minute
	case "1h", "1hour", "hour":
		return time.Hour
	case "1d", "1day", "day":
		return 24 * time.Hour
	case "1w", "1week", "week":
		return 7 * 24 * time.Hour
	case "1M", "1month", "month":
		return 30 * 24 * time.Hour // approximate; month buckets via 30d steps
	}
	return time.Hour
}

// bucketIdentity renders a bucket index back to its start instant: the grid
// point plus the offset (ES offset semantics), formatted with the layout.
func (p dateKeyPlan) bucketIdentity(key interface{}) (string, interface{}) {
	idx, ok := toInt64(key)
	if !ok {
		return fmt.Sprintf("%v", key), key
	}
	step := p.intervalStep()
	start := time.Unix(idx*int64(step.Seconds())+int64(p.offset.Seconds()), 0).UTC()
	layout := timeLayouts[p.format]
	if layout == "" {
		layout = "2006-01-02T15:04:05"
	}
	return start.Format(layout), start.UnixMilli()
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func (p dateKeyPlan) postProcess(buckets []api.Bucket) []api.Bucket {
	sortBucketsByTime(buckets)
	return zeroFill(buckets, p.interval, p.format)
}

// execBucketPartitioned runs ONE level query for the bucket agg, grouped by
// (ancestors..., key), partitions rows by ancestor tuple, assembles each
// partition's buckets (metrics/sort/truncate/fill), recursively executes
// bucket children (their level queries group by this level too), and runs
// post-pass children per bucket. Returns nodes keyed by ancestor tuple.
func (e *aggExecutor) execBucketPartitioned(agg api.Aggregation, ancestors []string, plan keyPlan) (map[string]*api.AggNode, error) {
	children := splitChildren(agg.GetNested())
	rows, err := e.levelQuery(plan, ancestors, children.metrics)
	if err != nil {
		return nil, err
	}

	// Partition rows by ancestor tuple.
	parts := map[string][]bucketRow{}
	for _, r := range rows {
		tk := tupleKey(r.ancestors)
		parts[tk] = append(parts[tk], r)
	}

	keyExpr := plan.keyExpr(e.resolve)
	childAncestors := append(append([]string{}, ancestors...), keyExpr)

	out := map[string]*api.AggNode{}
	for tk, prows := range parts {
		node := &api.AggNode{Buckets: []api.Bucket{}}
		// Tuple and source row per bucket key, so linkage and post-pass
		// scoping survive postProcess reordering/truncation.
		tupleByKey := make(map[string]string, len(prows))
		rowByKey := make(map[string]bucketRow, len(prows))
		for _, r := range prows {
			keyStr, keyRaw := plan.bucketIdentity(r.key)
			bucket := api.Bucket{Key: keyStr, KeyRaw: keyRaw, DocCount: r.docCount, Aggs: map[string]*api.AggNode{}}
			attachMetrics(bucket.Aggs, children.metrics, r.metrics)
			node.Buckets = append(node.Buckets, bucket)
			// Child partition keys are tupleKey over the child's ancestors —
			// this row's ancestors plus its own key.
			tupleByKey[valueKey(keyRaw)] = tupleKey(append(append([]interface{}{}, r.ancestors...), r.key))
			rowByKey[valueKey(keyRaw)] = r
		}
		node.Buckets = plan.postProcess(node.Buckets)
		fillEmptyBucketMetrics(node.Buckets, children.metrics)

		// Tuple/source-row lookup per surviving bucket; zero-filled buckets
		// have no source row and get empty child nodes.
		tupleOf := func(b api.Bucket) (string, bool) {
			t, ok := tupleByKey[valueKey(b.KeyRaw)]
			return t, ok
		}

		// Bucket children: one level query each, partitioned by child ancestors.
		for _, child := range children.buckets {
			childParts, err := e.execOnePartitioned(child, childAncestors)
			if err != nil {
				return nil, err
			}
			for i := range node.Buckets {
				tk2, ok := tupleOf(node.Buckets[i])
				if !ok {
					node.Buckets[i].Aggs[child.name] = &api.AggNode{Buckets: []api.Bucket{}}
					continue
				}
				if cn, ok := childParts[tk2]; ok {
					node.Buckets[i].Aggs[child.name] = cn
				} else {
					node.Buckets[i].Aggs[child.name] = &api.AggNode{Buckets: []api.Bucket{}}
				}
			}
		}

		// Post-pass children per bucket (scoped single-bucket queries).
		for i := range node.Buckets {
			row, ok := rowByKey[valueKey(node.Buckets[i].KeyRaw)]
			if !ok {
				continue
			}
			scoped := &extraScope{exprs: childAncestors, values: tupleValues(row)}
			for _, post := range children.post {
				switch pa := post.agg.(type) {
				case *api.PercentilesAggregation:
					vals, err := e.percentiles(pa, scoped)
					if err != nil {
						return nil, err
					}
					node.Buckets[i].Aggs[post.name] = &api.AggNode{Values: vals}
				case *api.TopHitsAggregation:
					n, err := e.topHits(pa, scoped)
					if err != nil {
						return nil, err
					}
					node.Buckets[i].Aggs[post.name] = n
				}
			}
		}

		out[tk] = node
	}
	return out, nil
}

// execOnePartitioned dispatches a bucket child to its partitioned execution.
func (e *aggExecutor) execOnePartitioned(child namedAgg, ancestors []string) (map[string]*api.AggNode, error) {
	switch a := child.agg.(type) {
	case *api.TermsAggregation:
		return e.execBucketPartitioned(a, ancestors, termsKeyPlan{field: a.Field, size: a.Size})
	case *api.DateHistogramAggregation:
		interval := a.Interval
		if interval == "" && a.IntervalField != "" {
			interval = a.IntervalField
		}
		format, ok := histogramFormatFor(interval)
		if !ok {
			warnAggUnsupported("date_histogram with interval " + interval)
			return map[string]*api.AggNode{}, nil
		}
		return e.execBucketPartitioned(a, ancestors, dateKeyPlan{field: a.Field, format: format, interval: interval, offset: a.Offset, epochExpr: e.epochExpr(a.Field)})
	case *api.AutoDateHistogramAggregation:
		dh, err := e.autoIntervalDH(a)
		if err != nil {
			return nil, err
		}
		return e.execOnePartitioned(namedAgg{child.name, dh}, ancestors)
	}
	return map[string]*api.AggNode{}, nil
}

// levelQuery runs the per-node query: GROUP BY (ancestors..., key) with the
// level's metric aggregates. NULL keys are omitted (ES semantics: missing
// doc values produce no bucket).
func (e *aggExecutor) levelQuery(plan keyPlan, ancestors []string, metrics []namedMetric) ([]bucketRow, error) {
	keyExpr := plan.keyExpr(e.resolve)

	var selects, groupBy []string
	for _, a := range ancestors {
		selects = append(selects, a)
		groupBy = append(groupBy, a)
	}
	selects = append(selects, keyExpr+" AS agg_key", "COUNT(*) AS agg_dc")
	groupBy = append(groupBy, "agg_key")

	metricExprs := make([]string, len(metrics))
	for i, m := range metrics {
		if fn, ok := metricSQL(m.agg, e.resolve); ok {
			metricExprs[i] = fn
		} else {
			metricExprs[i] = "NULL"
		}
		selects = append(selects, metricExprs[i])
	}

	cond := fmt.Sprintf("%s IS NOT NULL", keyExpr)
	if e.where != "" {
		cond = "(" + e.where + ") AND " + cond
	}

	q := fmt.Sprintf("SELECT %s FROM [%s] WHERE %s GROUP BY %s",
		strings.Join(selects, ", "), e.index, cond, strings.Join(groupBy, ", "))

	rows, err := e.handler.DB.Query(q, e.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []bucketRow
	nAnc := len(ancestors)
	for rows.Next() {
		anc := make([]interface{}, nAnc)
		scan := make([]interface{}, 0, nAnc+2+len(metrics))
		for i := range anc {
			scan = append(scan, &anc[i])
		}
		var key interface{}
		var dc int64
		scan = append(scan, &key, &dc)
		mvals := make([]sql.NullFloat64, len(metrics))
		for i := range mvals {
			scan = append(scan, &mvals[i])
		}
		if err := rows.Scan(scan...); err != nil {
			return nil, err
		}
		row := bucketRow{ancestors: anc, key: key, docCount: dc}
		for _, mv := range mvals {
			row.metrics = append(row.metrics, mv.Float64)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// metricSQL translates a metric spec to its SQL aggregate expression.
func metricSQL(m *api.MetricAggregation, resolve sqliteOrm.FieldResolver) (string, bool) {
	expr := sqliteOrm.ExprFor(resolve, m.Field)
	switch m.Type {
	case api.MetricCount:
		return fmt.Sprintf("COUNT(%s)", expr), true
	case api.MetricCardinality:
		return fmt.Sprintf("COUNT(DISTINCT %s)", expr), true
	case api.MetricSum:
		return fmt.Sprintf("SUM(%s)", expr), true
	case api.MetricAvg:
		return fmt.Sprintf("AVG(%s)", expr), true
	case api.MetricMin:
		return fmt.Sprintf("MIN(%s)", expr), true
	case api.MetricMax:
		return fmt.Sprintf("MAX(%s)", expr), true
	}
	return "", false
}

// attachMetrics writes the level query's metric values into a bucket scope.
func attachMetrics(scope map[string]*api.AggNode, metrics []namedMetric, vals []float64) {
	for i, m := range metrics {
		scope[m.name] = &api.AggNode{Value: vals[i], ValueSet: true}
	}
}

// fillEmptyBucketMetrics gives zero-filled buckets their sub-metric nodes,
// mirroring ES semantics on min_doc_count:0 buckets: count/sum/cardinality
// report 0; avg/min/max report no value.
func fillEmptyBucketMetrics(buckets []api.Bucket, metrics []namedMetric) {
	if len(metrics) == 0 {
		return
	}
	for i := range buckets {
		if buckets[i].DocCount != 0 || len(buckets[i].Aggs) > 0 {
			continue
		}
		if buckets[i].Aggs == nil {
			buckets[i].Aggs = map[string]*api.AggNode{}
		}
		for _, m := range metrics {
			switch m.agg.Type {
			case api.MetricAvg, api.MetricMin, api.MetricMax:
				buckets[i].Aggs[m.name] = &api.AggNode{}
			default: // count, sum, cardinality
				buckets[i].Aggs[m.name] = &api.AggNode{Value: 0, ValueSet: true}
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Post-pass and single-value executions
// ──────────────────────────────────────────────────────────────────────────

// extraScope pins ancestor values for per-bucket post passes.
type extraScope struct {
	exprs  []string
	values []interface{}
}

func (e *aggExecutor) scopedWhere(scope *extraScope) (string, []interface{}) {
	cond := e.where
	cargs := append([]interface{}{}, e.args...)
	if scope != nil && len(scope.exprs) > 0 && len(scope.exprs) == len(scope.values) {
		for i := range scope.exprs {
			eq := fmt.Sprintf("%s IS ?", scope.exprs[i])
			if cond == "" {
				cond = eq
			} else {
				cond = "(" + cond + ") AND " + eq
			}
			cargs = append(cargs, scope.values[i])
		}
	}
	if cond == "" {
		return "", cargs
	}
	return " WHERE " + cond, cargs
}

// tupleValues renders a row's full parent tuple (ancestors + own key).
func tupleValues(r bucketRow) []interface{} {
	return append(append([]interface{}{}, r.ancestors...), r.key)
}

func tupleKey(vals []interface{}) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = valueKey(v)
	}
	return strings.Join(parts, "\x00")
}

func valueKey(v interface{}) string { return fmt.Sprintf("%v", v) }

// topMetric computes a single-value metric over the whole scoped set.
func (e *aggExecutor) topMetric(a *api.MetricAggregation) (*float64, error) {
	fn, ok := metricSQL(a, e.resolve)
	if !ok {
		warnAggUnsupported(a.Type + " metric")
		return nil, nil
	}
	q := fmt.Sprintf("SELECT %s FROM [%s]", fn, e.index)
	if e.where != "" {
		q += " WHERE " + e.where
	}
	var v sql.NullFloat64
	if err := e.handler.DB.QueryRow(q, e.args...).Scan(&v); err != nil {
		return nil, fmt.Errorf("%s aggregation on %s: %w", a.Type, a.Field, err)
	}
	if !v.Valid {
		return nil, nil
	}
	f := v.Float64
	return &f, nil
}

// percentiles computes exact nearest-rank percentiles (ES uses approximate
// TDigest; conformance compares with tolerance).
func (e *aggExecutor) percentiles(a *api.PercentilesAggregation, scope *extraScope) (map[string]float64, error) {
	expr := sqliteOrm.ExprFor(e.resolve, a.Field)
	cond, cargs := e.scopedWhere(scope)

	var total int64
	if err := e.handler.DB.QueryRow(
		fmt.Sprintf("SELECT COUNT(%s) FROM [%s]%s", expr, e.index, cond), cargs...).Scan(&total); err != nil {
		return nil, err
	}
	out := map[string]float64{}
	if total == 0 {
		return out, nil
	}
	percents := a.Percents
	if len(percents) == 0 {
		percents = []float64{1, 5, 25, 50, 75, 95, 99}
	}
	for _, p := range percents {
		rank := int64(p/100.0*float64(total) + 0.5)
		if rank < 1 {
			rank = 1
		}
		if rank > total {
			rank = total
		}
		var v sql.NullFloat64
		q := fmt.Sprintf("SELECT %s FROM [%s]%s AND %s IS NOT NULL ORDER BY %s ASC LIMIT 1 OFFSET %d",
			expr, e.index, whereOrTrue(cond), expr, expr, rank-1)
		if err := e.handler.DB.QueryRow(q, cargs...).Scan(&v); err == nil && v.Valid {
			out[strconv.FormatFloat(p, 'f', -1, 64)] = v.Float64
		}
	}
	return out, nil
}

func whereOrTrue(cond string) string {
	if cond == "" {
		return " WHERE 1=1"
	}
	return cond
}

// topHits fetches the top document of the scoped set.
func (e *aggExecutor) topHits(a *api.TopHitsAggregation, scope *extraScope) (*api.AggNode, error) {
	orderBy, dir := "id", "DESC"
	if len(a.Sorts) > 0 {
		orderBy = sqliteOrm.ExprFor(e.resolve, a.Sorts[0].Field)
		if a.Sorts[0].SortType == api.ASC {
			dir = "ASC"
		}
	}
	cond, cargs := e.scopedWhere(scope)
	q := fmt.Sprintf("SELECT raw FROM [%s]%s ORDER BY %s %s LIMIT 1", e.index, cond, orderBy, dir)
	var raw []byte
	err := e.handler.DB.QueryRow(q, cargs...).Scan(&raw)
	if err == sql.ErrNoRows {
		return &api.AggNode{}, nil
	}
	if err != nil {
		return nil, err
	}
	doc := json.RawMessage(raw)
	return &api.AggNode{TopHit: &doc}, nil
}

// dateRange counts documents per [from, to) range.
func (e *aggExecutor) dateRange(a *api.DateRangeAggregation) (*api.AggNode, error) {
	expr := sqliteOrm.ExprFor(e.resolve, a.Field)
	buckets := []api.Bucket{}
	for _, r := range a.Ranges {
		rangeMap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		var extra string
		var extraArgs []interface{}
		if from, ok := rangeMap["from"]; ok && from != nil {
			extra = fmt.Sprintf("%s >= ?", expr)
			extraArgs = append(extraArgs, from)
		}
		if to, ok := rangeMap["to"]; ok && to != nil {
			if extra != "" {
				extra += " AND "
			}
			extra += fmt.Sprintf("%s < ?", expr)
			extraArgs = append(extraArgs, to)
		}
		if extra == "" {
			continue
		}
		cond := extra
		cargs := append([]interface{}{}, extraArgs...)
		if e.where != "" {
			cond = "(" + e.where + ") AND (" + extra + ")"
			cargs = append(append([]interface{}{}, e.args...), extraArgs...)
		}
		var count int64
		if err := e.handler.DB.QueryRow(
			fmt.Sprintf("SELECT COUNT(*) FROM [%s] WHERE %s", e.index, cond), cargs...).Scan(&count); err != nil {
			return nil, fmt.Errorf("date_range aggregation on %s: %w", a.Field, err)
		}
		bucket := api.Bucket{DocCount: count, Aggs: map[string]*api.AggNode{}}
		if from, ok := rangeMap["from"]; ok && from != nil {
			bucket.KeyRaw = from
		}
		if key, ok := rangeMap["key"]; ok && key != nil {
			bucket.Key = fmt.Sprintf("%v", key)
		} else {
			bucket.Key = fmt.Sprintf("%v-%v", rangeMap["from"], rangeMap["to"])
		}
		buckets = append(buckets, bucket)
	}
	return &api.AggNode{Buckets: buckets}, nil
}

// filterAgg runs a nested scope under an extra WHERE from the filter's
// query map (term/terms/range/bool.filter subset; anything else warns and
// yields an empty node — never a silent wrong result).
func (e *aggExecutor) filterAgg(a *api.FilterAggregation) (*api.AggNode, error) {
	clause, err := filterQueryToClause(a.Query)
	if err != nil || clause == nil {
		if err != nil {
			warnAggUnsupported("filter aggregation (" + err.Error() + ")")
		}
		return &api.AggNode{Buckets: []api.Bucket{{Aggs: map[string]*api.AggNode{}}}}, nil
	}
	qb := api.NewQuery().Filter(clause)
	qb.Build()
	fw, fa := sqliteOrm.BuildWhereClause(qb, e.resolve)

	combined, cargs := fw, append([]interface{}{}, fa...)
	if e.where != "" && fw != "" {
		combined = "(" + e.where + ") AND (" + fw + ")"
		cargs = append(append([]interface{}{}, e.args...), fa...)
	}

	sub := &aggExecutor{handler: e.handler, index: e.index, resolve: e.resolve, where: combined, args: cargs}
	return sub.singleBucketScope(a.GetNested())
}

// singleBucketScope executes a nested scope as a single-bucket node with
// the total doc count (sampler fallback).
func (e *aggExecutor) singleBucketScope(nested map[string]api.Aggregation) (*api.AggNode, error) {
	nodes, err := e.execute(nested)
	if err != nil {
		return nil, err
	}
	var dc int64
	q := fmt.Sprintf("SELECT COUNT(*) FROM [%s]", e.index)
	if e.where != "" {
		q += " WHERE " + e.where
	}
	_ = e.handler.DB.QueryRow(q, e.args...).Scan(&dc)
	return &api.AggNode{Buckets: []api.Bucket{{DocCount: dc, Aggs: nodes}}}, nil
}

// autoIntervalDH derives a fixed interval from the data range and returns
// an equivalent DateHistogramAggregation (design doc §4.2 fallback).
func (e *aggExecutor) autoIntervalDH(a *api.AutoDateHistogramAggregation) (*api.DateHistogramAggregation, error) {
	target := a.Buckets
	if target <= 0 {
		target = 10
	}
	expr := sqliteOrm.ExprFor(e.resolve, a.Field)
	var minV, maxV sql.NullString
	q := fmt.Sprintf("SELECT MIN(%s), MAX(%s) FROM [%s]", expr, expr, e.index)
	if e.where != "" {
		q += " WHERE " + e.where
	}
	if err := e.handler.DB.QueryRow(q, e.args...).Scan(&minV, &maxV); err != nil || !minV.Valid || !maxV.Valid {
		return &api.DateHistogramAggregation{Field: a.Field, Interval: "1h"}, nil
	}
	interval := pickInterval(minV.String, maxV.String, target, a.MinimumInterval)
	dh := &api.DateHistogramAggregation{Field: a.Field, Interval: interval}
	for subName, sub := range a.GetNested() {
		dh.AddNested(subName, sub)
	}
	return dh, nil
}

// pickInterval chooses a fixed interval covering the range in ~target
// buckets, respecting minimum_interval as the floor.
func pickInterval(minV, maxV string, target int, minimumInterval string) string {
	tMin, err1 := time.Parse(time.RFC3339, minV)
	tMax, err2 := time.Parse(time.RFC3339, maxV)
	if err1 != nil || err2 != nil || target <= 0 {
		return "1h"
	}
	candidate := tMax.Sub(tMin) / time.Duration(target)
	var floor time.Duration
	switch minimumInterval {
	case "minute":
		floor = time.Minute
	case "hour":
		floor = time.Hour
	case "day":
		floor = 24 * time.Hour
	case "week":
		floor = 7 * 24 * time.Hour
	case "month":
		floor = 30 * 24 * time.Hour
	}
	if floor > 0 && candidate < floor {
		candidate = floor
	}
	switch {
	case candidate < time.Hour:
		return "1m"
	case candidate < 24*time.Hour:
		return "1h"
	case candidate < 30*24*time.Hour:
		return "1d"
	default:
		return "1M"
	}
}

// filterQueryToClause translates the supported ES filter-query subset.
func filterQueryToClause(q map[string]interface{}) (*api.Clause, error) {
	if q == nil {
		return nil, nil
	}
	if term, ok := q["term"].(map[string]interface{}); ok && len(term) == 1 {
		for f, v := range term {
			return api.TermQuery(f, v), nil
		}
	}
	if terms, ok := q["terms"].(map[string]interface{}); ok && len(terms) == 1 {
		for f, v := range terms {
			if list, ok := v.([]interface{}); ok {
				return api.TermsQuery(f, list), nil
			}
		}
	}
	if rng, ok := q["range"].(map[string]interface{}); ok && len(rng) == 1 {
		for f, v := range rng {
			body, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			var clauses []*api.Clause
			if gte, ok := body["gte"]; ok {
				clauses = append(clauses, &api.Clause{Field: f, Operator: api.QueryRangeGte, Value: gte})
			}
			if gt, ok := body["gt"]; ok {
				clauses = append(clauses, &api.Clause{Field: f, Operator: api.QueryRangeGt, Value: gt})
			}
			if lte, ok := body["lte"]; ok {
				clauses = append(clauses, &api.Clause{Field: f, Operator: api.QueryRangeLte, Value: lte})
			}
			if lt, ok := body["lt"]; ok {
				clauses = append(clauses, &api.Clause{Field: f, Operator: api.QueryRangeLt, Value: lt})
			}
			if len(clauses) == 1 {
				return clauses[0], nil
			}
			if len(clauses) > 1 {
				return api.MustQuery(clauses...), nil
			}
		}
	}
	if b, ok := q["bool"].(map[string]interface{}); ok {
		if filters, ok := b["filter"].([]interface{}); ok {
			var clauses []*api.Clause
			for _, f := range filters {
				fm, ok := f.(map[string]interface{})
				if !ok {
					continue
				}
				c, err := filterQueryToClause(fm)
				if err != nil {
					return nil, err
				}
				if c != nil {
					clauses = append(clauses, c)
				}
			}
			if len(clauses) == 1 {
				return clauses[0], nil
			}
			if len(clauses) > 1 {
				return api.MustQuery(clauses...), nil
			}
			return nil, nil
		}
	}
	return nil, fmt.Errorf("unsupported filter query %v", keysOf(q))
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ──────────────────────────────────────────────────────────────────────────
// Time-bucket helpers
// ──────────────────────────────────────────────────────────────────────────

// histogramFormatFor maps interval strings to strftime bucket formats.
func histogramFormatFor(interval string) (string, bool) {
	switch strings.TrimSpace(interval) {
	case "1m", "1minute", "minute":
		return "%Y-%m-%dT%H:%M:00", true
	case "1h", "1hour", "hour":
		return "%Y-%m-%dT%H:00:00", true
	case "1d", "1day", "day":
		return "%Y-%m-%dT00:00:00", true
	case "1M", "1month", "month":
		return "%Y-%m-01T00:00:00", true
	}
	return "", false
}

var timeLayouts = map[string]string{
	"%Y-%m-%dT%H:%M:00": "2006-01-02T15:04:00",
	"%Y-%m-%dT%H:00:00": "2006-01-02T15:00:00",
	"%Y-%m-%dT00:00:00": "2006-01-02T00:00:00",
	"%Y-%m-01T00:00:00": "2006-01-02T00:00:00",
}

// epochOf converts a formatted bucket key to epoch milliseconds.
func epochOf(key, format string) int64 {
	layout, ok := timeLayouts[format]
	if !ok {
		return 0
	}
	t, err := time.Parse(layout, key)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

func sortBucketsByTime(buckets []api.Bucket) {
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Key < buckets[j].Key })
}

// stepOfInterval returns the zero-fill step (0 = no fill).
func stepOfInterval(interval string) time.Duration {
	switch strings.TrimSpace(interval) {
	case "1m", "1minute", "minute":
		return time.Minute
	case "1h", "1hour", "hour":
		return time.Hour
	case "1d", "1day", "day":
		return 24 * time.Hour
	}
	return 0
}

// zeroFill inserts empty buckets between the first and last key (ES
// min_doc_count:0 + extended_bounds semantics; fixed intervals only).
// Bucket identity comes from KeyRaw (epoch ms) — string layouts stay
// presentation-only.
func zeroFill(buckets []api.Bucket, interval, format string) []api.Bucket {
	step := stepOfInterval(interval)
	if step == 0 || len(buckets) < 2 {
		return buckets
	}
	layout := timeLayouts[format]
	if layout == "" {
		layout = "2006-01-02T15:04:05"
	}
	filled := make([]api.Bucket, 0, len(buckets)*2)
	for i, b := range buckets {
		filled = append(filled, b)
		if i == len(buckets)-1 {
			break
		}
		cur, ok1 := toInt64(b.KeyRaw)
		next, ok2 := toInt64(buckets[i+1].KeyRaw)
		if !ok1 || !ok2 {
			continue
		}
		for t := cur + step.Milliseconds(); t < next; t += step.Milliseconds() {
			ts := time.UnixMilli(t).UTC()
			filled = append(filled, api.Bucket{
				Key: ts.Format(layout), KeyRaw: t, DocCount: 0, Aggs: map[string]*api.AggNode{},
			})
		}
	}
	return filled
}

// ──────────────────────────────────────────────────────────────────────────
// ES-shape conversion (SearchV2 backward compatibility)
// ──────────────────────────────────────────────────────────────────────────

// typedToESShape converts the typed aggregation tree to the ES-shaped maps
// SearchV2 has always returned, so existing consumers are unaffected while
// Aggregate callers get the typed model.
func typedToESShape(nodes map[string]*api.AggNode) map[string]interface{} {
	out := map[string]interface{}{}
	for name, node := range nodes {
		out[name] = nodeToESShape(node)
	}
	return out
}

func nodeToESShape(node *api.AggNode) map[string]interface{} {
	if node == nil {
		return map[string]interface{}{}
	}
	out := map[string]interface{}{}
	if node.Values != nil {
		out["values"] = node.Values
	}
	if node.TopHit != nil {
		var doc interface{}
		_ = json.Unmarshal(*node.TopHit, &doc)
		out["hits"] = map[string]interface{}{
			"hits": map[string]interface{}{"hits": []interface{}{map[string]interface{}{"_source": doc}}},
		}
	}
	if node.ValueSet {
		out["value"] = node.Value
	}
	if node.Buckets != nil {
		buckets := make([]interface{}, 0, len(node.Buckets))
		for _, b := range node.Buckets {
			bm := map[string]interface{}{}
			if b.Key != "" {
				bm["key"] = b.Key
			}
			if b.KeyRaw != nil {
				bm["key_raw"] = b.KeyRaw
				// Time buckets: numeric epoch-ms key + string form (ES).
				if ms, ok := b.KeyRaw.(int64); ok && ms > 0 {
					bm["key"] = ms
					bm["key_as_string"] = b.Key
				}
			}
			bm["doc_count"] = b.DocCount
			for subName, subNode := range b.Aggs {
				bm[subName] = nodeToESShape(subNode)
			}
			buckets = append(buckets, bm)
		}
		// Single-bucket aggs (filter/sampler) inline their scope ES-style.
		if len(node.Buckets) == 1 && node.Buckets[0].Key == "" && node.Buckets[0].KeyRaw == nil {
			single := map[string]interface{}{"doc_count": node.Buckets[0].DocCount}
			for subName, subNode := range node.Buckets[0].Aggs {
				single[subName] = nodeToESShape(subNode)
			}
			for k, v := range single {
				out[k] = v
			}
			return out
		}
		out["buckets"] = buckets
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────────
// Unsupported-feature warnings (grace period: warn once, empty result)
// ──────────────────────────────────────────────────────────────────────────

var aggUnsupportedOnce sync.Map

func warnAggUnsupported(kind string) {
	if _, loaded := aggUnsupportedOnce.LoadOrStore(kind, true); !loaded {
		log.Warnf("sqlite orm: %s aggregation is not supported on this backend; it returns an empty result (grace period: warning only)", kind)
	}
}

func aggKind(agg api.Aggregation) string {
	switch agg.(type) {
	case *api.FilterAggregation:
		return "filter"
	case *api.PercentilesAggregation:
		return "percentiles"
	case *api.PipelineAggregation:
		return "pipeline"
	}
	return fmt.Sprintf("%T", agg)
}
