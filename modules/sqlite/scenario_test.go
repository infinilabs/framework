/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

// ──────────────────────────────────────────────────────────────────────────
// Comprehensive query & aggregation scenario (oracle-based verification).
//
// A realistic observability-events dataset exercises the full QueryBuilder
// operator matrix, boolean composition, promoted (generated-column) and
// unpromoted (json_extract fallback) paths, FTS text search, sorting and
// pagination, and the complete aggregation surface incl. pipelines. Expected
// results are computed by independent Go oracles over the in-memory fixture
// — the test validates the SQL machinery against straightforward reference
// implementations, not against hand-written constants.
//
// The suite drives the handler methods directly (no global orm.Register),
// so it coexists with the other sqlite tests in one binary.
// ──────────────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
)

// ── scenario model ─────────────────────────────────────────────────────────

type svcInfo struct {
	Name    string `json:"name,omitempty" elastic_mapping:"name:{type:keyword}"`
	Version string `json:"version,omitempty" elastic_mapping:"version:{type:keyword}"`
}

type scenarioEvent struct {
	orm.ORMObjectBase
	Stream   string    `json:"stream" elastic_mapping:"stream:{type:keyword}"`
	Severity string    `json:"severity" elastic_mapping:"severity:{type:keyword}"`
	Region   string    `json:"region" elastic_mapping:"region:{type:keyword}"`
	Host     string    `json:"host" elastic_mapping:"host:{type:keyword}"`
	Message  string    `json:"message" elastic_mapping:"message:{type:text}"`
	Status   int       `json:"status" elastic_mapping:"status:{type:integer}"`
	Latency  float64   `json:"latency" elastic_mapping:"latency:{type:double}"`
	Verified bool      `json:"verified" elastic_mapping:"verified:{type:boolean}"`
	TS       time.Time `json:"ts" elastic_mapping:"ts:{type:date}"`
	Svc      *svcInfo  `json:"svc,omitempty" elastic_mapping:"svc:{type:object}"`
	// Extra carries dynamic fields with NO mapping — queries on extra.*
	// exercise the json_extract fallback path.
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// ── fixture ────────────────────────────────────────────────────────────────

type scenario struct {
	t       *testing.T
	handler *SQLiteORM
	events  []scenarioEvent // in-memory oracle copy, insertion order
	byID    map[string]*scenarioEvent
}

var (
	scStreams    = []string{"checkout", "search", "auth", "billing"}
	scSeverities = []string{"info", "warning", "error", "fatal"}
	scRegions    = []string{"cn-east", "cn-north", "us-west"}
	scHosts      = []string{"node-1", "node-2", "node-3", "node-4", "node-5"}
	scServices   = []svcInfo{
		{Name: "api-gateway", Version: "v1"},
		{Name: "api-gateway", Version: "v2"},
		{Name: "order-svc", Version: "v1"},
		{Name: "user-svc", Version: "v3"},
	}
	scMessages = []string{
		"request completed successfully",
		"connection reset by peer",
		"timeout while waiting for upstream",
		"disk usage above threshold",
		"authentication failed for user",
		"database connection pool exhausted",
		"rate limit exceeded for tenant",
		"healthy heartbeat received",
	}
)

func newScenario(t *testing.T) *scenario {
	t.Helper()
	handler := &SQLiteORM{Config: SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(t.TempDir(), "scenario.db"),
	}}
	require.NoError(t, handler.Open())
	t.Cleanup(func() { handler.Close() })
	require.NoError(t, handler.RegisterSchemaWithName(scenarioEvent{}, "scenario_events"))

	s := &scenario{t: t, handler: handler, byID: map[string]*scenarioEvent{}}
	rng := rand.New(rand.NewSource(20260813))

	// Deterministic span: 2026-08-07..2026-08-13 (7 days). Timestamps are
	// built absolutely (not additively) so minutes never overflow across
	// hour/day boundaries into the engineered gaps.
	for i := 0; i < 800; i++ {
		// Skewed hour distribution with guaranteed gaps (zero-fill checks):
		// skip hours 10..14 on day 2 and hour 3 on day 5.
		day := rng.Intn(7)
		hour := rng.Intn(24)
		if day == 2 && hour >= 10 && hour <= 14 {
			hour = 15
		}
		if day == 5 && hour == 3 {
			hour = 4
		}
		minute := rng.Intn(60)
		second := rng.Intn(60)
		ts := time.Date(2026, 8, 7+day, hour, minute, second, 0, time.UTC)
		sev := scSeverities[rng.Intn(len(scSeverities))]
		if rng.Intn(10) == 0 {
			sev = "fatal" // boost fatal to a deterministic minority
		}
		ev := scenarioEvent{
			Stream:   scStreams[rng.Intn(len(scStreams))],
			Severity: sev,
			Region:   scRegions[rng.Intn(len(scRegions))],
			Host:     scHosts[rng.Intn(len(scHosts))],
			Message:  scMessages[rng.Intn(len(scMessages))],
			Status:   100 + rng.Intn(500),              // [100,600)
			Latency:  float64(rng.Intn(90000)) / 100.0, // [0,900) ms, 2dp
			Verified: rng.Intn(2) == 0,
			TS:       ts,
			Svc:      &scServices[rng.Intn(len(scServices))],
			Extra: map[string]interface{}{
				"tenant":  fmt.Sprintf("tenant-%d", rng.Intn(6)),
				"attempt": float64(rng.Intn(3) + 1),
			},
		}
		ev.ID = fmt.Sprintf("ev-%04d", i)
		require.NoError(t, handler.Save(nil, &ev))
		s.events = append(s.events, ev)
		s.byID[ev.ID] = &s.events[len(s.events)-1]
	}
	return s
}

func (s *scenario) ctx() *orm.Context {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &scenarioEvent{})
	return ctx
}

// ── query oracle ───────────────────────────────────────────────────────────

// query runs qb through SearchV2 and returns the decoded hits.
func (s *scenario) query(qb *orm.QueryBuilder) []scenarioEvent {
	s.t.Helper()
	qb.Build() // idempotent-safe: Build is invoked again by SearchV2
	res, err := s.handler.SearchV2(s.ctx(), qb)
	require.NoError(s.t, err)
	hits, _, err := elastic.DecodeHits[scenarioEvent](res)
	require.NoError(s.t, err)
	return hits
}

func (s *scenario) queryIDs(qb *orm.QueryBuilder) []string {
	s.t.Helper()
	hits := s.query(qb)
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	sort.Strings(ids)
	return ids
}

// expect returns the sorted IDs of fixture events matching pred (the oracle).
func (s *scenario) expect(pred func(*scenarioEvent) bool) []string {
	ids := []string{}
	for i := range s.events {
		if pred(&s.events[i]) {
			ids = append(ids, s.events[i].ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func (s *scenario) expectIDs(pred func(*scenarioEvent) bool) map[string]struct{} {
	out := map[string]struct{}{}
	for _, id := range s.expect(pred) {
		out[id] = struct{}{}
	}
	return out
}

// tokenize mirrors FTS5 unicode61 tokenization for the fixture vocabulary
// (lowercase words split on spaces/punctuation).
func scTokenize(msg string) []string {
	return strings.FieldsFunc(strings.ToLower(msg), func(r rune) bool {
		return !('a' <= r && r <= 'z' || '0' <= r && r <= '9')
	})
}

func scHasAnyWord(msg string, words []string) bool {
	tokens := map[string]bool{}
	for _, t := range scTokenize(msg) {
		tokens[t] = true
	}
	for _, w := range words {
		if tokens[strings.ToLower(w)] {
			return true
		}
	}
	return false
}

func scHasPhrase(msg, phrase string) bool {
	return strings.Contains(strings.ToLower(msg), strings.ToLower(phrase))
}

// ── query matrix ───────────────────────────────────────────────────────────

func TestScenario_Queries(t *testing.T) {
	s := newScenario(t)

	t.Run("term keyword", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.TermQuery("stream", "checkout")))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return e.Stream == "checkout" }), got)
	})

	t.Run("terms multi-value", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.TermsQuery("severity", []interface{}{"error", "fatal"})))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return e.Severity == "error" || e.Severity == "fatal"
		}), got)
	})

	t.Run("range int gte lt", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().
			Filter(orm.Range("status").Gte(300)).
			Filter(orm.Range("status").Lt(400)))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return e.Status >= 300 && e.Status < 400
		}), got)
	})

	t.Run("range float gte", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.Range("latency").Gte(700.5)))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return e.Latency >= 700.5 }), got)
	})

	t.Run("range date", func(t *testing.T) {
		cutoff := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
		got := s.queryIDs(orm.NewQuery().Filter(orm.Range("ts").Gte(cutoff.Format(time.RFC3339))))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return !e.TS.Before(cutoff) }), got)
	})

	t.Run("bool term", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.TermQuery("verified", false)))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return !e.Verified }), got)
	})

	t.Run("prefix", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(&orm.Clause{Field: "host", Operator: orm.QueryPrefix, Value: "node-1"}))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return strings.HasPrefix(e.Host, "node-1") }), got)
	})

	t.Run("wildcard", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(&orm.Clause{Field: "host", Operator: orm.QueryWildcard, Value: "node-?"}))
		// node-? matches single-char suffix: node-1..node-5 all match via
		// '?' → '_', i.e. hosts "node-X" — all five hosts.
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return len(e.Host) == 6 // "node-X"
		}), got)
	})

	t.Run("exists nested object", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(&orm.Clause{Field: "svc", Operator: orm.QueryExists}))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return e.Svc != nil }), got)
	})

	t.Run("must_not", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.MustNotQuery(orm.TermQuery("region", "cn-east"))))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return e.Region != "cn-east" }), got)
	})

	t.Run("should minimum_should_match 1", func(t *testing.T) {
		qb := orm.NewQuery().
			Filter(orm.TermQuery("stream", "auth")).
			Must(orm.ShouldQuery(
				orm.TermQuery("region", "cn-east"),
				orm.TermQuery("region", "us-west"),
			).Parameter("minimum_should_match", 1))
		got := s.queryIDs(qb)
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return e.Stream == "auth" && (e.Region == "cn-east" || e.Region == "us-west")
		}), got)
	})

	t.Run("nested dotted path term", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.TermQuery("svc.name", "order-svc")))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return e.Svc != nil && e.Svc.Name == "order-svc" }), got)
	})

	t.Run("unmapped dynamic path term (json_extract fallback)", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.TermQuery("extra.tenant", "tenant-3")))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool { return e.Extra["tenant"] == "tenant-3" }), got)
	})

	t.Run("unmapped dynamic numeric range", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.Range("extra.attempt").Gte(3)))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			v, _ := e.Extra["attempt"].(float64)
			return v >= 3
		}), got)
	})

	t.Run("fulltext match single word", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.MatchQuery("message", "timeout")))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return scHasAnyWord(e.Message, []string{"timeout"})
		}), got)
	})

	t.Run("fulltext match multi word OR semantics", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(orm.MatchQuery("message", "timeout disk")))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return scHasAnyWord(e.Message, []string{"timeout", "disk"})
		}), got)
	})

	t.Run("fulltext match phrase", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(&orm.Clause{
			Field: "message", Operator: orm.QueryMatchPhrase, Value: "connection reset",
		}))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return scHasPhrase(e.Message, "connection reset")
		}), got)
	})

	t.Run("fulltext query_string phrase", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(&orm.Clause{
			Field: "message", Operator: orm.QueryQueryString, Value: "pool exhausted",
		}))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return scHasPhrase(e.Message, "pool exhausted")
		}), got)
	})

	t.Run("multi_match text+keyword", func(t *testing.T) {
		got := s.queryIDs(orm.NewQuery().Filter(&orm.Clause{
			Field: "message,host", Operator: orm.QueryMultiMatch, Value: "node-3",
		}))
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			return strings.Contains(e.Host, "node-3") || scHasAnyWord(e.Message, []string{"node-3"})
		}), got)
	})

	t.Run("composite: everything combined", func(t *testing.T) {
		cutoff := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
		qb := orm.NewQuery().
			Filter(orm.TermsQuery("severity", []interface{}{"error", "fatal"})).
			Filter(orm.Range("status").Gte(200)).
			Filter(orm.Range("latency").Lt(500)).
			Filter(orm.Range("ts").Gte(cutoff.Format(time.RFC3339))).
			Filter(orm.MustNotQuery(orm.TermQuery("region", "us-west"))).
			Must(orm.ShouldQuery(
				orm.TermQuery("svc.name", "api-gateway"),
				orm.TermQuery("svc.name", "user-svc"),
			).Parameter("minimum_should_match", 1))
		got := s.queryIDs(qb)
		assert.ElementsMatch(t, s.expect(func(e *scenarioEvent) bool {
			if e.Severity != "error" && e.Severity != "fatal" {
				return false
			}
			if e.Status < 200 || e.Latency >= 500 || e.TS.Before(cutoff) || e.Region == "us-west" {
				return false
			}
			return e.Svc != nil && (e.Svc.Name == "api-gateway" || e.Svc.Name == "user-svc")
		}), got)
	})

	t.Run("sort multi-key with pagination", func(t *testing.T) {
		qb := func(from, size int) *orm.QueryBuilder {
			return orm.NewQuery().
				Filter(orm.TermQuery("stream", "search")).
				SortBy(orm.Sort{Field: "status", SortType: orm.ASC},
					orm.Sort{Field: "ts", SortType: orm.DESC}).
				From(from).Size(size)
		}
		// Oracle ordering: status asc, ts desc (stable insertion tiebreak).
		type ev = *scenarioEvent
		ordered := make([]ev, 0, len(s.events))
		for id := range s.expectIDs(func(e *scenarioEvent) bool { return e.Stream == "search" }) {
			ordered = append(ordered, s.byID[id])
		}
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].Status != ordered[j].Status {
				return ordered[i].Status < ordered[j].Status
			}
			return ordered[i].TS.After(ordered[j].TS)
		})

		page1 := s.query(qb(0, 10))
		require.Len(t, page1, 10)
		for i := 0; i < 10; i++ {
			assert.Equal(t, ordered[i].ID, page1[i].ID, "page1[%d]", i)
		}
		page2 := s.query(qb(10, 10))
		require.Len(t, page2, 10)
		for i := 0; i < 10; i++ {
			assert.Equal(t, ordered[10+i].ID, page2[i].ID, "page2[%d]", i)
		}
	})

	t.Run("pagination from without size", func(t *testing.T) {
		all := s.query(orm.NewQuery().Filter(orm.TermQuery("stream", "auth")))
		rest := s.query(orm.NewQuery().Filter(orm.TermQuery("stream", "auth")).From(5))
		require.Len(t, rest, len(all)-5)
	})

	t.Run("unsupported operators warn and match nothing", func(t *testing.T) {
		for _, op := range []orm.QueryType{orm.QuerySemantic, orm.QueryHybrid, orm.QueryNested} {
			res, err := s.handler.SearchV2(s.ctx(), orm.NewQuery().Filter(&orm.Clause{
				Field: "message", Operator: op, Value: "x",
			}))
			require.NoError(t, err, "%s must not error (grace period)", op)
			hits, _, err := elastic.DecodeHits[scenarioEvent](res)
			require.NoError(t, err)
			assert.Empty(t, hits, "%s matches nothing", op)
		}
	})
}

// ── aggregation matrix ─────────────────────────────────────────────────────

func (s *scenario) aggregate(qb *orm.QueryBuilder) *orm.AggregationResult {
	s.t.Helper()
	if qb == nil {
		qb = orm.NewQuery()
	}
	res, err := s.handler.Aggregate(s.ctx(), qb)
	require.NoError(s.t, err)
	require.NotNil(s.t, res)
	return res
}

func feq(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestScenario_Aggregations(t *testing.T) {
	s := newScenario(t)

	t.Run("metrics full set", func(t *testing.T) {
		var wantCount, wantSum float64
		wantMin, wantMax := math.MaxFloat64, -math.MaxFloat64
		distinct := map[float64]bool{}
		for i := range s.events {
			v := s.events[i].Latency
			wantCount++
			wantSum += v
			if v < wantMin {
				wantMin = v
			}
			if v > wantMax {
				wantMax = v
			}
			distinct[v] = true
		}
		m := func(typ, field string) *orm.MetricAggregation {
			return &orm.MetricAggregation{Type: typ, Field: field}
		}
		qb := orm.NewQuery()
		qb.SetAggregations(map[string]orm.Aggregation{
			"count": m(orm.MetricCount, "latency"),
			"sum":   m(orm.MetricSum, "latency"),
			"avg":   m(orm.MetricAvg, "latency"),
			"min":   m(orm.MetricMin, "latency"),
			"max":   m(orm.MetricMax, "latency"),
			"card":  m(orm.MetricCardinality, "latency"),
		})
		res := s.aggregate(qb)
		assert.True(t, feq(res.Aggs["count"].Value, wantCount))
		assert.True(t, feq(res.Aggs["sum"].Value, wantSum))
		assert.True(t, feq(res.Aggs["avg"].Value, wantSum/wantCount))
		assert.True(t, feq(res.Aggs["min"].Value, wantMin))
		assert.True(t, feq(res.Aggs["max"].Value, wantMax))
		assert.True(t, feq(res.Aggs["card"].Value, float64(len(distinct))))
	})

	t.Run("terms with nested metric", func(t *testing.T) {
		terms := &orm.TermsAggregation{Field: "severity", Size: 10}
		terms.AddNested("sum_latency", &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"})
		terms.AddNested("avg_status", &orm.MetricAggregation{Type: orm.MetricAvg, Field: "status"})
		qb := orm.NewQuery()
		qb.SetAggs("by_sev", terms)
		res := s.aggregate(qb)

		want := map[string]struct {
			count int
			sumL  float64
			sumS  float64
		}{}
		for i := range s.events {
			w := want[s.events[i].Severity]
			w.count++
			w.sumL += s.events[i].Latency
			w.sumS += float64(s.events[i].Status)
			want[s.events[i].Severity] = w
		}
		buckets := res.Aggs["by_sev"].Buckets
		require.Len(t, buckets, len(want))
		for _, b := range buckets {
			w, ok := want[b.Key]
			require.True(t, ok, "unexpected bucket %q", b.Key)
			assert.EqualValues(t, w.count, b.DocCount)
			assert.True(t, feq(b.Aggs["sum_latency"].Value, w.sumL), "%s sum_latency", b.Key)
			assert.True(t, feq(b.Aggs["avg_status"].Value, w.sumS/float64(w.count)), "%s avg_status", b.Key)
		}
	})

	t.Run("terms size truncation keeps top by count", func(t *testing.T) {
		terms := &orm.TermsAggregation{Field: "stream", Size: 2}
		qb := orm.NewQuery()
		qb.SetAggs("top_streams", terms)
		res := s.aggregate(qb)
		buckets := res.Aggs["top_streams"].Buckets
		require.Len(t, buckets, 2)
		assert.GreaterOrEqual(t, buckets[0].DocCount, buckets[1].DocCount, "count desc ordering")
	})

	t.Run("three-level nesting stream→severity→metric", func(t *testing.T) {
		streams := &orm.TermsAggregation{Field: "stream", Size: 10}
		sevs := &orm.TermsAggregation{Field: "severity", Size: 10}
		sevs.AddNested("total", &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"})
		streams.AddNested("by_sev", sevs)
		qb := orm.NewQuery()
		qb.SetAggs("streams", streams)
		res := s.aggregate(qb)

		type sevAgg struct {
			count int
			sumL  float64
		}
		want := map[string]map[string]sevAgg{}
		for i := range s.events {
			e := &s.events[i]
			if want[e.Stream] == nil {
				want[e.Stream] = map[string]sevAgg{}
			}
			w := want[e.Stream][e.Severity]
			w.count++
			w.sumL += e.Latency
			want[e.Stream][e.Severity] = w
		}
		require.Len(t, res.Aggs["streams"].Buckets, len(want))
		for _, sb := range res.Aggs["streams"].Buckets {
			inner := want[sb.Key]
			require.NotNil(t, inner, "stream %q", sb.Key)
			sevNode := sb.Aggs["by_sev"]
			require.NotNil(t, sevNode)
			require.Len(t, sevNode.Buckets, len(inner))
			for _, pb := range sevNode.Buckets {
				w := inner[pb.Key]
				assert.EqualValues(t, w.count, pb.DocCount, "%s/%s", sb.Key, pb.Key)
				assert.True(t, feq(pb.Aggs["total"].Value, w.sumL), "%s/%s sum", sb.Key, pb.Key)
			}
		}
	})

	t.Run("date_histogram daily with zero fill", func(t *testing.T) {
		dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1d"}
		dh.AddNested("count", &orm.MetricAggregation{Type: orm.MetricCount, Field: "latency"})
		qb := orm.NewQuery()
		qb.SetAggs("days", dh)
		res := s.aggregate(qb)

		want := map[string]int{}
		for i := range s.events {
			day := s.events[i].TS.UTC().Format("2006-01-02")
			want[day]++
		}
		buckets := res.Aggs["days"].Buckets
		require.Len(t, buckets, len(want))
		var total int64
		for _, b := range buckets {
			day := strings.TrimSuffix(b.Key, "T00:00:00")
			assert.EqualValues(t, want[day], b.DocCount, "day %s", day)
			total += b.DocCount
		}
		assert.EqualValues(t, len(s.events), total)
	})

	t.Run("date_histogram hourly zero fill gaps", func(t *testing.T) {
		// Scope to day 2 hours 8..18: fixture guarantees hours 10-14 empty.
		from := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
		to := time.Date(2026, 8, 9, 19, 0, 0, 0, time.UTC)
		dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
		qb := orm.NewQuery().
			Filter(orm.Range("ts").Gte(from.Format(time.RFC3339))).
			Filter(orm.Range("ts").Lte(to.Format(time.RFC3339)))
		qb.SetAggs("hours", dh)
		res := s.aggregate(qb)

		buckets := res.Aggs["hours"].Buckets
		// 8..18 inclusive = 11 hours, all present (data or zero-filled).
		require.Len(t, buckets, 11)
		for h := 10; h <= 14; h++ {
			key := fmt.Sprintf("2026-08-09T%02d:00:00", h)
			var found bool
			for _, b := range buckets {
				if b.Key == key {
					found = true
					assert.EqualValues(t, 0, b.DocCount, "gap hour %s must be zero-filled", key)
				}
			}
			assert.True(t, found, "gap hour %s present", key)
		}
	})

	t.Run("date_histogram offset preserves totals", func(t *testing.T) {
		// Offset shifts boundaries, not membership: total docs invariant.
		offset := 37*time.Minute + 12*time.Second
		dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h", Offset: offset}
		qb := orm.NewQuery()
		qb.SetAggs("h", dh)
		res := s.aggregate(qb)
		var total int64
		for _, b := range res.Aggs["h"].Buckets {
			total += b.DocCount
		}
		assert.EqualValues(t, len(s.events), total)
	})

	t.Run("date_histogram monthly", func(t *testing.T) {
		dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1M"}
		qb := orm.NewQuery()
		qb.SetAggs("months", dh)
		res := s.aggregate(qb)
		buckets := res.Aggs["months"].Buckets
		require.Len(t, buckets, 1) // all 7 days within 2026-08
		assert.EqualValues(t, len(s.events), buckets[0].DocCount)
	})

	t.Run("auto_date_histogram interval derivation", func(t *testing.T) {
		// The fallback heuristic picks ceil-ish fixed intervals: a ~7d span
		// with Buckets=6 → candidate ≈ 28h → 1d buckets; with Buckets=7 the
		// candidate is 23.98h → 1h buckets (documented heuristic behavior).
		adh := &orm.AutoDateHistogramAggregation{Field: "ts", Buckets: 6}
		qb := orm.NewQuery()
		qb.SetAggs("auto", adh)
		res := s.aggregate(qb)
		buckets := res.Aggs["auto"].Buckets
		require.NotEmpty(t, buckets)
		assert.InDelta(t, 8, len(buckets), 1.1, "daily buckets over the 7d span")
		var total int64
		for _, b := range buckets {
			total += b.DocCount
		}
		assert.EqualValues(t, len(s.events), total)
	})

	t.Run("date_range", func(t *testing.T) {
		d1 := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
		dr := &orm.DateRangeAggregation{Field: "ts", Ranges: []interface{}{
			map[string]interface{}{"from": "2026-08-07T00:00:00Z", "to": d1.Format(time.RFC3339), "key": "day0"},
			map[string]interface{}{"from": d1.Format(time.RFC3339), "key": "rest"},
		}}
		qb := orm.NewQuery()
		qb.SetAggs("ranges", dr)
		res := s.aggregate(qb)
		buckets := res.Aggs["ranges"].Buckets
		require.Len(t, buckets, 2)
		wantDay0, wantRest := 0, 0
		for i := range s.events {
			if s.events[i].TS.Before(d1) {
				wantDay0++
			} else {
				wantRest++
			}
		}
		assert.EqualValues(t, wantDay0, buckets[0].DocCount)
		assert.EqualValues(t, wantRest, buckets[1].DocCount)
	})

	t.Run("filter bucket term + nested sum", func(t *testing.T) {
		filter := &orm.FilterAggregation{Query: map[string]interface{}{
			"term": map[string]interface{}{"severity": "fatal"},
		}}
		filter.AddNested("sum_latency", &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"})
		qb := orm.NewQuery()
		qb.SetAggs("fatals", filter)
		res := s.aggregate(qb)
		buckets := res.Aggs["fatals"].Buckets
		require.Len(t, buckets, 1)
		var wantCount, wantSum float64
		for i := range s.events {
			if s.events[i].Severity == "fatal" {
				wantCount++
				wantSum += s.events[i].Latency
			}
		}
		assert.EqualValues(t, wantCount, buckets[0].DocCount)
		assert.True(t, feq(buckets[0].Aggs["sum_latency"].Value, wantSum))
	})

	t.Run("filter bucket with range query", func(t *testing.T) {
		filter := &orm.FilterAggregation{Query: map[string]interface{}{
			"range": map[string]interface{}{"status": map[string]interface{}{"gte": 500}},
		}}
		qb := orm.NewQuery()
		qb.SetAggs("high_status", filter)
		res := s.aggregate(qb)
		want := 0
		for i := range s.events {
			if s.events[i].Status >= 500 {
				want++
			}
		}
		assert.EqualValues(t, want, res.Aggs["high_status"].Buckets[0].DocCount)
	})

	t.Run("percentiles within rank tolerance", func(t *testing.T) {
		p := &orm.PercentilesAggregation{Field: "latency", Percents: []float64{50, 95}}
		qb := orm.NewQuery()
		qb.SetAggs("p", p)
		res := s.aggregate(qb)
		vals := res.Aggs["p"].Values
		require.NotEmpty(t, vals)

		all := make([]float64, 0, len(s.events))
		for i := range s.events {
			all = append(all, s.events[i].Latency)
		}
		sort.Float64s(all)
		rank := func(pct float64) float64 {
			idx := int(pct / 100 * float64(len(all)))
			if idx >= len(all) {
				idx = len(all) - 1
			}
			return all[idx]
		}
		// Exact nearest-rank; allow the neighboring ranks as tolerance.
		for _, pct := range []float64{50, 95} {
			want := rank(pct)
			lo, hi := all[max(0, int(pct/100*float64(len(all)))-1)], all[min(len(all)-1, int(pct/100*float64(len(all)))+1)]
			got := vals[fmt.Sprintf("%g", pct)]
			assert.GreaterOrEqual(t, got, lo-1e-6, "p%v within rank tolerance", pct)
			assert.LessOrEqual(t, got, hi+1e-6, "p%v within rank tolerance", pct)
			_ = want
		}
	})

	t.Run("top_hits per stream latest by ts", func(t *testing.T) {
		streams := &orm.TermsAggregation{Field: "stream", Size: 10}
		streams.AddNested("latest", &orm.TopHitsAggregation{
			Size:  1,
			Sorts: []orm.Sort{{Field: "ts", SortType: orm.DESC}},
		})
		qb := orm.NewQuery()
		qb.SetAggs("streams", streams)
		res := s.aggregate(qb)
		latest := map[string]*scenarioEvent{}
		for i := range s.events {
			e := &s.events[i]
			if cur := latest[e.Stream]; cur == nil || e.TS.After(cur.TS) {
				latest[e.Stream] = e
			}
		}
		for _, sb := range res.Aggs["streams"].Buckets {
			node := sb.Aggs["latest"]
			require.NotNil(t, node, "stream %s", sb.Key)
			require.NotNil(t, node.TopHit, "stream %s top hit", sb.Key)
			var doc map[string]interface{}
			require.NoError(t, jsonUnmarshal(*node.TopHit, &doc))
			// Tie-break by id: both the SQL (ORDER BY ts DESC) and the oracle
			// pick a max-ts doc; with equal timestamps either is acceptable.
			assert.Equal(t, latest[sb.Key].TS.Format(time.RFC3339), doc["ts"], "stream %s", sb.Key)
		}
	})

	t.Run("pipelines on daily histogram", func(t *testing.T) {
		dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1d"}
		dh.AddNested("sum_latency", &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"})
		dh.AddNested("count", &orm.MetricAggregation{Type: orm.MetricCount, Field: "latency"})
		dh.AddNested("deriv", &orm.DerivativeAggregation{BucketsPath: "count"})
		dh.AddNested("avg_expr", &orm.BucketScriptAggregation{
			BucketsPath: map[string]string{"s": "sum_latency", "c": "count"},
			Script:      "params.s / params.c",
		})
		qb := orm.NewQuery()
		qb.SetAggregations(map[string]orm.Aggregation{
			"days":  dh,
			"total": &orm.PipelineAggregation{Type: orm.MetricSumBucket, BucketsPath: "days>count"},
			"peak":  &orm.MaxBucketAggregation{BucketsPath: "days>count"},
		})
		res := s.aggregate(qb)

		var counts []float64
		var sums []float64
		for _, b := range res.Aggs["days"].Buckets {
			counts = append(counts, b.Aggs["count"].Value)
			sums = append(sums, b.Aggs["sum_latency"].Value)
		}
		require.Len(t, counts, 7)
		// derivative[0] unset (ES null semantics — the node may be absent);
		// diffs match from bucket 1 on.
		if d := res.Aggs["days"].Buckets[0].Aggs["deriv"]; d != nil {
			assert.False(t, d.ValueSet)
		}
		for i := 1; i < len(counts); i++ {
			assert.True(t, feq(res.Aggs["days"].Buckets[i].Aggs["deriv"].Value, counts[i]-counts[i-1]), "deriv[%d]", i)
			assert.True(t, feq(res.Aggs["days"].Buckets[i].Aggs["avg_expr"].Value, sums[i]/counts[i]), "avg_expr[%d]", i)
		}
		var wantSum, wantPeak float64
		for _, c := range counts {
			wantSum += c
			if c > wantPeak {
				wantPeak = c
			}
		}
		assert.True(t, feq(res.Aggs["total"].Value, wantSum))
		assert.True(t, feq(res.Aggs["peak"].Value, wantPeak))
	})

	t.Run("bucket_sort top-k by sub metric", func(t *testing.T) {
		streams := &orm.TermsAggregation{Field: "stream", Size: 10}
		streams.AddNested("total", &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"})
		streams.AddNested("sort", &orm.BucketSortAggregation{
			Sort: []orm.BucketSortSpec{{Path: "total", Desc: true}},
			Size: 2,
		})
		qb := orm.NewQuery()
		qb.SetAggs("streams", streams)
		res := s.aggregate(qb)

		totals := map[string]float64{}
		for i := range s.events {
			totals[s.events[i].Stream] += s.events[i].Latency
		}
		type kv struct {
			k string
			v float64
		}
		ranked := make([]kv, 0, len(totals))
		for k, v := range totals {
			ranked = append(ranked, kv{k, v})
		}
		sort.Slice(ranked, func(i, j int) bool { return ranked[i].v > ranked[j].v })

		buckets := res.Aggs["streams"].Buckets
		require.GreaterOrEqual(t, len(buckets), 2)
		for i := 0; i < 2; i++ {
			assert.Equal(t, ranked[i].k, buckets[i].Key, "rank %d", i)
		}
	})

	t.Run("deep chain with sum_bucket per stream", func(t *testing.T) {
		streams := &orm.TermsAggregation{Field: "stream", Size: 10}
		days := &orm.DateHistogramAggregation{Field: "ts", Interval: "1d"}
		days.AddNested("sum_latency", &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"})
		streams.AddNested("days", days)
		streams.AddNested("grand", &orm.PipelineAggregation{Type: orm.MetricSumBucket, BucketsPath: "days>sum_latency"})
		qb := orm.NewQuery()
		qb.SetAggs("streams", streams)
		res := s.aggregate(qb)

		want := map[string]float64{}
		for i := range s.events {
			want[s.events[i].Stream] += s.events[i].Latency
		}
		for _, sb := range res.Aggs["streams"].Buckets {
			assert.True(t, feq(sb.Aggs["grand"].Value, want[sb.Key]), "stream %s grand total", sb.Key)
		}
	})

	t.Run("terms on nested dotted path", func(t *testing.T) {
		terms := &orm.TermsAggregation{Field: "svc.name", Size: 10}
		qb := orm.NewQuery()
		qb.SetAggs("svcs", terms)
		res := s.aggregate(qb)
		want := map[string]int{}
		for i := range s.events {
			want[s.events[i].Svc.Name]++
		}
		require.Len(t, res.Aggs["svcs"].Buckets, len(want))
		for _, b := range res.Aggs["svcs"].Buckets {
			assert.EqualValues(t, want[b.Key], b.DocCount, "svc %s", b.Key)
		}
	})

	t.Run("terms on unmapped dynamic path", func(t *testing.T) {
		terms := &orm.TermsAggregation{Field: "extra.tenant", Size: 10}
		qb := orm.NewQuery()
		qb.SetAggs("tenants", terms)
		res := s.aggregate(qb)
		want := map[string]int{}
		for i := range s.events {
			t := s.events[i].Extra["tenant"].(string)
			want[t]++
		}
		require.Len(t, res.Aggs["tenants"].Buckets, len(want))
		for _, b := range res.Aggs["tenants"].Buckets {
			assert.EqualValues(t, want[b.Key], b.DocCount, "tenant %s", b.Key)
		}
	})

	t.Run("aggregation respects query filter", func(t *testing.T) {
		terms := &orm.TermsAggregation{Field: "severity", Size: 10}
		qb := orm.NewQuery().Filter(orm.TermQuery("stream", "auth"))
		qb.SetAggs("sevs", terms)
		res := s.aggregate(qb)
		want := map[string]int{}
		for i := range s.events {
			if s.events[i].Stream == "auth" {
				want[s.events[i].Severity]++
			}
		}
		require.Len(t, res.Aggs["sevs"].Buckets, len(want))
		for _, b := range res.Aggs["sevs"].Buckets {
			assert.EqualValues(t, want[b.Key], b.DocCount)
		}
	})

	t.Run("empty set behaviors", func(t *testing.T) {
		sum := &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"}
		terms := &orm.TermsAggregation{Field: "severity", Size: 10}
		qb := orm.NewQuery().Filter(orm.TermQuery("stream", "no-such-stream"))
		qb.SetAggs("total", sum, "sevs", terms)
		res := s.aggregate(qb)
		assert.False(t, res.Aggs["total"].ValueSet, "sum over empty set has no value")
		assert.Empty(t, res.Aggs["sevs"].Buckets)
	})
}

// ── cross-path consistency ─────────────────────────────────────────────────

func TestScenario_CrossPathConsistency(t *testing.T) {
	s := newScenario(t)

	t.Run("SearchV2 ES-shape == Aggregate typed", func(t *testing.T) {
		terms := &orm.TermsAggregation{Field: "severity", Size: 10}
		terms.AddNested("total", &orm.MetricAggregation{Type: orm.MetricSum, Field: "latency"})
		qb := orm.NewQuery().Filter(orm.Range("status").Gte(300))
		qb.SetAggs("by_sev", terms)

		// Typed path.
		typed := s.aggregate(qb)
		esMap := map[string]map[string]float64{}
		for _, b := range typed.Aggs["by_sev"].Buckets {
			esMap[b.Key] = map[string]float64{
				"doc_count": float64(b.DocCount),
				"total":     b.Aggs["total"].Value,
			}
		}

		// ES-shaped path (SearchV2 side channel).
		res, err := s.handler.SearchV2(s.ctx(), qb)
		require.NoError(t, err)
		resp, err := elastic.DecodeSearchResult(res)
		require.NoError(t, err)
		require.NotNil(t, resp.Aggregations)
		bySev, ok := resp.Aggregations["by_sev"]
		require.True(t, ok)
		require.Len(t, bySev.Buckets, len(esMap))
		for _, b := range bySev.Buckets {
			key := fmt.Sprintf("%v", b["key"])
			want := esMap[key]
			require.NotNil(t, want, "bucket %v", key)
			assert.EqualValues(t, want["doc_count"], b["doc_count"], "%v doc_count", key)
			if sub, ok := b["total"].(map[string]interface{}); ok {
				assert.True(t, feq(want["total"], sub["value"].(float64)), "%v total", key)
			}
		}
	})

	t.Run("promoted and unmapped paths agree on identical data", func(t *testing.T) {
		// region is promoted (generated column); extra.mirror holds the same
		// values with no mapping. Both paths must select identically.
		for i := range s.events {
			s.events[i].Extra["mirror"] = s.events[i].Region
		}
		for i := range s.events {
			raw := fmt.Sprintf(`{"extra":{"tenant":%q,"mirror":%q}}`, s.events[i].Extra["tenant"], s.events[i].Region)
			_, err := s.handler.DB.Exec("UPDATE scenario_events SET raw = json_set(raw, '$.extra.mirror', ?) WHERE id = ?",
				s.events[i].Region, s.events[i].ID)
			require.NoError(t, err)
			_ = raw
		}
		viaPromoted := s.queryIDs(orm.NewQuery().Filter(orm.TermQuery("region", "cn-north")))
		viaFallback := s.queryIDs(orm.NewQuery().Filter(orm.TermQuery("extra.mirror", "cn-north")))
		assert.ElementsMatch(t, viaPromoted, viaFallback)
	})
}

// small helpers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func jsonUnmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, v) }
