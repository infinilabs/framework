/* Copyright © INFINI LTD. All rights reserved. */

// Package aggregate hosts the backend-independent aggregation machinery:
// the pipeline engine (derivative / sum_bucket / max_bucket / bucket_script /
// bucket_sort) and shared result-shaping helpers (zero fill, ordering).
//
// Bucket and metric aggregations are computed by each backend natively;
// pipelines are pure second-order derivations over the returned bucket tree,
// so they are computed here exactly once — every backend behaves identically
// (design doc §4.1). Backends call ApplyPipelines before returning from
// MetricsAPI.Aggregate.
package aggregate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"infini.sh/framework/core/orm"
)

// ApplyPipelines fills the pipeline aggregation nodes of result, guided by
// the aggregation spec tree (mirroring qb.Aggs). Bucket/metric nodes must
// already be populated; ES-native pipeline values, when present, are
// overwritten so backends cannot disagree.
func ApplyPipelines(res *orm.AggregationResult, aggs map[string]orm.Aggregation) error {
	if res == nil {
		return nil
	}
	return applyScope(res.Aggs, aggs)
}

// applyScope processes one naming scope: recurses into bucket aggregations
// first, then evaluates sibling pipelines (sum_bucket, max_bucket) against
// the completed scope.
func applyScope(nodes map[string]*orm.AggNode, spec map[string]orm.Aggregation) error {
	if nodes == nil || spec == nil {
		return nil
	}
	for name, node := range nodes {
		if node != nil && len(node.Buckets) > 0 {
			if err := applyBucketList(node.Buckets, nestedSpecOf(spec[name])); err != nil {
				return err
			}
		}
	}
	for name, sp := range spec {
		switch s := sp.(type) {
		// sibling pipelines create their node when the backend did not
		case *orm.PipelineAggregation:
			if s.Type != orm.MetricSumBucket {
				continue
			}
			vals, err := bucketValues(nodes, s.BucketsPath)
			if err != nil {
				return fmt.Errorf("sum_bucket %q: %w", name, err)
			}
			ensureNode(nodes, name).Value, _ = sum(vals), true
			nodes[name].ValueSet = true
		case *orm.MaxBucketAggregation:
			vals, err := bucketValues(nodes, s.BucketsPath)
			if err != nil {
				return fmt.Errorf("max_bucket %q: %w", name, err)
			}
			ensureNode(nodes, name).Value = max(vals)
			nodes[name].ValueSet = true
		}
	}
	return nil
}

// ensureNode returns the named node, creating it when absent (sibling
// pipeline results may not be pre-created by the backend).
func ensureNode(nodes map[string]*orm.AggNode, name string) *orm.AggNode {
	node, ok := nodes[name]
	if !ok || node == nil {
		node = &orm.AggNode{}
		nodes[name] = node
	}
	return node
}

// applyBucketList processes the buckets of one multi-bucket aggregation:
// each bucket's inner scope first, then the parent pipelines declared among
// its children (derivative, bucket_script, bucket_sort operate on the list).
func applyBucketList(buckets []orm.Bucket, childSpec map[string]orm.Aggregation) error {
	if childSpec == nil {
		return nil
	}
	for i := range buckets {
		if err := applyScope(buckets[i].Aggs, childSpec); err != nil {
			return err
		}
	}
	for name, sp := range childSpec {
		switch s := sp.(type) {
		case *orm.DerivativeAggregation:
			applyDerivative(buckets, name, s.BucketsPath)
		case *orm.BucketScriptAggregation:
			if err := applyBucketScript(buckets, name, s); err != nil {
				return fmt.Errorf("bucket_script %q: %w", name, err)
			}
		case *orm.BucketSortAggregation:
			applyBucketSort(buckets, name, s)
		}
	}
	return nil
}

// applyDerivative fills v[i] - v[i-1] of the referenced path per bucket; the
// first bucket gets no value (ES semantics: null).
func applyDerivative(buckets []orm.Bucket, name, bucketsPath string) {
	var prev float64
	prevSet := false
	for i := range buckets {
		cur, ok := scalarOf(&buckets[i], bucketsPath)
		if i > 0 && ok && prevSet {
			setNode(buckets[i].Aggs, name, cur-prev, true, true)
		}
		if ok {
			prev, prevSet = cur, true
		}
	}
}

// applyBucketScript evaluates the script per bucket over params resolved
// from the bucket's scope.
func applyBucketScript(buckets []orm.Bucket, name string, s *orm.BucketScriptAggregation) error {
	for i := range buckets {
		params := map[string]float64{}
		for p, path := range s.BucketsPath {
			v, ok := scalarOf(&buckets[i], path)
			if !ok {
				params[p] = 0 // ES: missing param → script yields null; we use 0 and let division guards apply
			}
			params[p] = v
		}
		val, err := EvalScript(s.Script, params)
		if err != nil {
			return err
		}
		setNode(buckets[i].Aggs, name, val, true, true)
	}
	return nil
}

// applyBucketSort reorders the bucket list by the first sort criterion and
// truncates to From/Size. Sorting is stable to keep deterministic output.
func applyBucketSort(buckets []orm.Bucket, name string, s *orm.BucketSortAggregation) {
	if len(s.Sort) == 0 || len(buckets) == 0 {
		return
	}
	crit := s.Sort[0]
	less := func(a, b *orm.Bucket) bool {
		av, aok := scalarOf(a, crit.Path)
		bv, bok := scalarOf(b, crit.Path)
		if !aok {
			return false
		}
		if !bok {
			return true
		}
		if crit.Desc {
			return av > bv
		}
		return av < bv
	}
	sort.SliceStable(buckets, func(i, j int) bool { return less(&buckets[i], &buckets[j]) })
	if s.From > 0 || s.Size > 0 {
		from := s.From
		if from > len(buckets) {
			from = len(buckets)
		}
		end := len(buckets)
		if s.Size > 0 && from+s.Size < end {
			end = from + s.Size
		}
		trimmed := make([]orm.Bucket, end-from)
		copy(trimmed, buckets[from:end])
		// bucket_sort lives among the children spec — it has no own node; the
		// trim mutates the shared slice header, so copy back.
		for i := range trimmed {
			buckets[i] = trimmed[i]
		}
		for i := len(trimmed); i < len(buckets); i++ {
			buckets[i] = orm.Bucket{}
		}
	}
	_ = name
}

// scalarOf resolves a buckets_path to a scalar within one bucket's scope:
// "_count"/"._count" → doc_count, "metric" → sibling value,
// "a>b" chains descend single-bucket aggs (rare; supported one level).
func scalarOf(bucket *orm.Bucket, path string) (float64, bool) {
	if bucket == nil {
		return 0, false
	}
	if path == "_count" || path == "._count" {
		return float64(bucket.DocCount), true
	}
	parts := strings.Split(path, ">")
	nodes := bucket.Aggs
	for i, part := range parts {
		if nodes == nil {
			return 0, false
		}
		node, ok := nodes[part]
		if !ok || node == nil {
			return 0, false
		}
		if i == len(parts)-1 {
			return node.Value, node.ValueSet
		}
		// intermediate: descend into its first bucket (single-bucket agg)
		if len(node.Buckets) == 0 {
			return 0, false
		}
		nodes = node.Buckets[0].Aggs
	}
	return 0, false
}

// bucketValues resolves "aggName>metric" against a scope: the per-bucket
// values of metric across all buckets of aggName.
func bucketValues(nodes map[string]*orm.AggNode, path string) ([]float64, error) {
	parts := strings.Split(strings.TrimSpace(path), ">")
	if len(parts) != 2 {
		// single-name path: values of a sibling metric across... sibling
		// metrics are single values, not lists — require the two-part form.
		return nil, fmt.Errorf("unsupported buckets_path %q (want \"agg>metric\")", path)
	}
	aggNode, ok := nodes[parts[0]]
	if !ok || aggNode == nil {
		return nil, fmt.Errorf("aggregation %q not found", parts[0])
	}
	out := make([]float64, 0, len(aggNode.Buckets))
	for i := range aggNode.Buckets {
		if v, ok := scalarOf(&aggNode.Buckets[i], parts[1]); ok {
			out = append(out, v)
		}
	}
	return out, nil
}

func setNode(scope map[string]*orm.AggNode, name string, val float64, set, ok bool) {
	if scope == nil {
		return
	}
	node := scope[name]
	if node == nil {
		node = &orm.AggNode{}
		scope[name] = node
	}
	node.Value = val
	node.ValueSet = set && ok
}

func nestedSpecOf(spec orm.Aggregation) map[string]orm.Aggregation {
	if spec == nil {
		return nil
	}
	return spec.GetNested()
}

func sum(vals []float64) float64 {
	var s float64
	for _, v := range vals {
		s += v
	}
	return s
}

func max(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// ──────────────────────────────────────────────────────────────────────────
// Script evaluation: arithmetic over params.* with + - * / ( ) and unary -.
// Covers the console vocabulary (ratios, percentages, scale factors); a real
// painless runtime is deliberately out of scope.
// ──────────────────────────────────────────────────────────────────────────

type scriptParser struct {
	toks   []string
	pos    int
	params map[string]float64
}

// EvalScript evaluates an arithmetic script with params.* references.
func EvalScript(script string, params map[string]float64) (float64, error) {
	p := &scriptParser{toks: tokenizeScript(script), params: params}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos != len(p.toks) {
		return 0, fmt.Errorf("unexpected token %q in script %q", p.toks[p.pos], script)
	}
	return v, nil
}

func tokenizeScript(s string) []string {
	var toks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		case strings.ContainsRune("+-*/()", r):
			flush()
			toks = append(toks, string(r))
		case r == '.':
			// A dot between digits is a decimal point ("1.5"); otherwise it
			// is the member-access separator ("params.a").
			digitBefore := i > 0 && runes[i-1] >= '0' && runes[i-1] <= '9'
			digitAfter := i+1 < len(runes) && runes[i+1] >= '0' && runes[i+1] <= '9'
			if digitBefore && digitAfter {
				cur.WriteRune(r)
			} else {
				flush()
				toks = append(toks, ".")
			}
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return toks
}

func (p *scriptParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func (p *scriptParser) next() string {
	t := p.peek()
	p.pos++
	return t
}

func (p *scriptParser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peek()
		if op != "+" && op != "-" {
			return left, nil
		}
		p.next()
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == "+" {
			left += right
		} else {
			left -= right
		}
	}
}

func (p *scriptParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peek()
		if op != "*" && op != "/" {
			return left, nil
		}
		p.next()
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == "*" {
			left *= right
		} else if right == 0 {
			// Console Calc convention: division by zero yields 0 — keep
			// evaluating the rest of the expression ("a / b * 100" with b=0
			// stays 0 rather than aborting the parse).
			left = 0
		} else {
			left /= right
		}
	}
}

func (p *scriptParser) parseFactor() (float64, error) {
	t := p.next()
	switch {
	case t == "":
		return 0, fmt.Errorf("unexpected end of script")
	case t == "(":
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.next() != ")" {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		return v, nil
	case t == "-":
		v, err := p.parseFactor()
		return -v, err
	case t == "params":
		if p.next() != "." {
			return 0, fmt.Errorf("expected '.' after params")
		}
		name := p.next()
		v, ok := p.params[name]
		if !ok {
			return 0, fmt.Errorf("unknown param %q", name)
		}
		return v, nil
	default:
		v, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, fmt.Errorf("unexpected token %q", t)
		}
		return v, nil
	}
}
