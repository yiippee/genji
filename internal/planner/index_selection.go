package planner

import (
	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/internal/expr"
	"github.com/genjidb/genji/internal/sql/scanner"
	"github.com/genjidb/genji/internal/stream"
)

// SelectIndex attempts to replace a sequential scan by an index scan or a pk scan by
// analyzing the stream for indexable filter nodes.
// It expects the first node of the stream to be a table.Scan.
//
// Compatibility of filter nodes.
//
// For a filter node to be selected if must be of the following form:
//   <path> <compatible operator> <expression>
// or
//   <expression> <compatible operator> <path>
// path: path of a document
// compatible operator: one of =, >, >=, <, <=, IN
// expression: any expression
//
// Index compatibility.
//
// Once we have a list of all compatible filter nodes, we try to associate
// indexes with them.
// Given the following index:
//   CREATE INDEX foo_a_idx ON foo (a)
// and this query:
//   SELECT * FROM foo WHERE a > 5 AND b > 10
//   table.Scan('foo') | docs.Filter(a > 5) | docs.Filter(b > 10) | docs.Project(*)
// foo_a_idx matches docs.Filter(a > 5) and can be selected.
// Now, with a different index:
//   CREATE INDEX foo_a_b_c_idx ON foo(a, b, c)
// and this query:
//   SELECT * FROM foo WHERE a > 5 AND c > 20
//   table.Scan('foo') | docs.Filter(a > 5) | docs.Filter(c > 20) | docs.Project(*)
// foo_a_b_c_idx matches with the first filter because a is the leftmost path indexed by it.
// The second filter is not selected because it is not the second leftmost path.
// For composite indexes, filter nodes can be selected if they match with one or more indexed path
// consecutively, from left to right.
// Now, let's have a look a this query:
//   SELECT * FROM foo WHERE a = 5 AND b = 10 AND c > 15 AND d > 20
//   table.Scan('foo') | docs.Filter(a = 5) | docs.Filter(b = 10) | docs.Filter(c > 15) | docs.Filter(d > 20) | docs.Project(*)
// foo_a_b_c_idx matches with first three filters because they satisfy several conditions:
// - each of them matches with the first 3 indexed paths, consecutively.
// - the first 2 filters use the equal operator
// A counter-example:
//   SELECT * FROM foo WHERE a = 5 AND b > 10 AND c > 15 AND d > 20
//   table.Scan('foo') | docs.Filter(a = 5) | docs.Filter(b > 10) | docs.Filter(c > 15) | docs.Filter(d > 20) | docs.Project(*)
// foo_a_b_c_idx only matches with the first two filter nodes because while the first node uses the equal
// operator, the second one doesn't, and thus the third node cannot be selected as well.
//
// Candidates and cost
//
// Because a table can have multiple indexes, we need to establish which of these
// indexes should be used to run the query, if not all of them.
// For that we generate a cost for each selected index and return the one with the cheapest cost.
func SelectIndex(sctx *StreamContext) error {
	// Lookup the seq scan node.
	// We will assume that at this point
	// if there is one it has to be the
	// first node of the stream.
	firstNode := sctx.Stream.First()
	if firstNode == nil {
		return nil
	}
	seq, ok := firstNode.(*stream.TableScanOperator)
	if !ok {
		return nil
	}

	// ensure the table exists
	_, err := sctx.Catalog.GetTableInfo(seq.TableName)
	if err != nil {
		return err
	}

	// ensure the list of filter nodes is not empty
	if len(sctx.Filters) == 0 {
		return nil
	}

	is := indexSelector{
		tableScan: seq,
		sctx:      sctx,
	}

	return is.selectIndex()
}

// indexSelector analyses a stream and generates a plan for each of them that
// can benefit from using an index.
// It then compares the cost of each plan and returns the cheapest stream.
type indexSelector struct {
	tableScan *stream.TableScanOperator
	sctx      *StreamContext
}

func (i *indexSelector) selectIndex() error {
	// generate a list of candidates from all the filter nodes that
	// can benefit from reading from an index or the table pk
	nodes := make(filterNodes, 0, len(i.sctx.Filters))
	for _, f := range i.sctx.Filters {
		filter := i.isFilterIndexable(f)
		if filter == nil {
			continue
		}

		nodes = append(nodes, filter)
	}

	// select the cheapest plan
	var selected *candidate
	var cost int

	// start with the primary key of the table
	tb, err := i.sctx.Catalog.GetTableInfo(i.tableScan.TableName)
	if err != nil {
		return err
	}
	pk := tb.GetPrimaryKey()
	if pk != nil {
		selected = i.associateIndexWithNodes(tb.TableName, false, false, pk.Paths, nodes)
		if selected != nil {
			cost = selected.Cost()
		}
	}

	// get all the indexes for this table and associate them
	// with compatible candidates
	for _, idxName := range i.sctx.Catalog.ListIndexes(i.tableScan.TableName) {
		idxInfo, err := i.sctx.Catalog.GetIndexInfo(idxName)
		if err != nil {
			return err
		}

		candidate := i.associateIndexWithNodes(idxInfo.IndexName, true, idxInfo.Unique, idxInfo.Paths, nodes)

		if candidate == nil {
			continue
		}

		if selected == nil {
			selected = candidate
			cost = selected.Cost()
			continue
		}

		c := candidate.Cost()

		if len(selected.nodes) < len(candidate.nodes) || (len(selected.nodes) == len(candidate.nodes) && c < cost) {
			cost = c
			selected = candidate
		}
	}

	if selected == nil {
		return nil
	}

	// remove the filter nodes from the tree
	for _, f := range selected.nodes {
		i.sctx.removeFilterNode(f.node.(*stream.DocsFilterOperator))
	}

	// we replace the seq scan node by the selected root
	s := i.sctx.Stream
	s.Remove(s.First())
	for i := len(selected.replaceRootBy) - 1; i >= 0; i-- {
		if s.Op == nil {
			s.Op = selected.replaceRootBy[i]
			continue
		}
		stream.InsertBefore(s.First(), selected.replaceRootBy[i])
	}
	i.sctx.Stream = s

	return nil
}

func (i *indexSelector) isFilterIndexable(f *stream.DocsFilterOperator) *filterNode {
	// only operators can associate this node to an index
	op, ok := f.Expr.(expr.Operator)
	if !ok {
		return nil
	}

	// ensure the operator is compatible
	if !operatorIsIndexCompatible(op) {
		return nil
	}

	// determine if the operator could benefit from an index
	ok, path, e := operatorCanUseIndex(op)
	if !ok {
		return nil
	}

	node := filterNode{
		node:     f,
		path:     path,
		operator: op.Token(),
		operand:  e,
	}

	return &node
}

// for a given index, select all filter nodes that match according to the following rules:
// - from left to right, associate each indexed path to a filter node and stop when there is no
// node available or the node is not compatible
// - for n associated nodes, the n - 1 first must all use the = operator, only the last one
// can be any of =, >, >=, <, <=
// - transform all associated nodes into an index range
// If not all indexed paths have an associated filter node, return whatever has been associated
// A few examples for this index: CREATE INDEX ON foo(a, b, c)
//   fitler(a = 3) | docs.Filter(b = 10) | (c > 20)
//   -> range = {min: [3, 10, 20]}
//   fitler(a = 3) | docs.Filter(b > 10) | (c > 20)
//   -> range = {min: [3], exact: true}
//  docs.Filter(a IN (1, 2))
//   -> ranges = [1], [2]
func (i *indexSelector) associateIndexWithNodes(treeName string, isIndex bool, isUnique bool, paths []document.Path, nodes filterNodes) *candidate {
	found := make([]*filterNode, 0, len(paths))

	var hasIn bool
	for _, p := range paths {
		n := nodes.getByPath(p)
		if n == nil {
			break
		}

		if n.operator == scanner.IN {
			hasIn = true
		}

		// in the case there is an IN operator somewhere
		// we only select additional IN or = operators.
		// Otherwise, any operator is accepted
		if !hasIn || (n.operator == scanner.EQ || n.operator == scanner.IN) {
			found = append(found, n)
		}

		// we must stop at the first operator that is not a IN or a =
		if n.operator != scanner.EQ && n.operator != scanner.IN {
			break
		}
	}

	if len(found) == 0 {
		return nil
	}

	// in case there is an IN operator in the list, we need to generate multiple ranges.
	// If not, we only need one range.
	var ranges stream.Ranges

	if !hasIn {
		ranges = stream.Ranges{i.buildRangeFromFilterNodes(found...)}
	} else {
		ranges = i.buildRangesFromFilterNodes(paths, found)
	}

	c := candidate{
		nodes:      found,
		rangesCost: ranges.Cost(),
		isIndex:    isIndex,
		isUnique:   isUnique,
	}

	if !isIndex {
		c.replaceRootBy = []stream.Operator{
			stream.TableScan(treeName, ranges...),
		}
	} else {
		c.replaceRootBy = []stream.Operator{
			stream.IndexScan(treeName, ranges...),
		}
	}

	return &c
}

func (i *indexSelector) buildRangesFromFilterNodes(paths []document.Path, filters []*filterNode) stream.Ranges {
	// build a 2 dimentional list of all expressions
	// so that: docs.Filter(a IN (10, 11)) | docs.Filter(b = 20) | docs.Filter(c IN (30, 31))
	// becomes:
	// [10, 11]
	// [20]
	// [30, 31]

	l := make([][]expr.Expr, 0, len(filters))

	for _, f := range filters {
		var row []expr.Expr
		if f.operator != scanner.IN {
			row = []expr.Expr{f.operand}
		} else {
			row = f.operand.(expr.LiteralExprList)
		}

		l = append(l, row)
	}

	// generate a list of combinaison between each row of the list
	// Example for the list above:
	// 10, 20, 30
	// 10, 20, 31
	// 11, 20, 30
	// 11, 20, 31

	var ranges stream.Ranges

	i.walkExpr(l, func(row []expr.Expr) {
		ranges = append(ranges, i.buildRangeFromOperator(scanner.EQ, paths[:len(row)], row...))
	})

	return ranges
}

func (i *indexSelector) walkExpr(l [][]expr.Expr, fn func(row []expr.Expr)) {
	curLine := l[0]

	if len(l) == 0 {
		return
	}

	if len(l) == 1 {
		for _, e := range curLine {
			fn([]expr.Expr{e})
		}

		return
	}

	for _, e := range curLine {
		i.walkExpr(l[1:], func(row []expr.Expr) {
			fn(append([]expr.Expr{e}, row...))
		})
	}
}

func (i *indexSelector) buildRangeFromFilterNodes(filters ...*filterNode) stream.Range {
	// first, generate a list of paths and a list of expressions
	paths := make([]document.Path, 0, len(filters))
	el := make(expr.LiteralExprList, 0, len(filters))
	for i := range filters {
		paths = append(paths, filters[i].path)
		el = append(el, filters[i].operand)
	}

	// use last filter node to determine the direction of the range
	filter := filters[len(filters)-1]

	return i.buildRangeFromOperator(filter.operator, paths, el...)
}

func (i *indexSelector) buildRangeFromOperator(lastOp scanner.Token, paths []document.Path, operands ...expr.Expr) stream.Range {
	rng := stream.Range{
		Paths: paths,
	}

	el := expr.LiteralExprList(operands)

	switch lastOp {
	case scanner.EQ, scanner.IN:
		rng.Exact = true
		rng.Min = el
	case scanner.GT:
		rng.Exclusive = true
		rng.Min = el
	case scanner.GTE:
		rng.Min = el
	case scanner.LT:
		rng.Exclusive = true
		rng.Max = el
	case scanner.LTE:
		rng.Max = el
	case scanner.BETWEEN:
		/* example:
		CREATE TABLE test(a int, b int, c int, d int, e int);
		CREATE INDEX on test(a, b, c, d);
		EXPLAIN SELECT * FROM test WHERE a = 1 AND b = 10 AND c = 100 AND d BETWEEN 1000 AND 2000 AND e > 10000;
		{
		    "plan": 'index.Scan("test_a_b_c_d_idx", [{"min": [1, 10, 100, 1000], "max": [1, 10, 100, 2000]}]) | docs.Filter(e > 10000)'
		}
		*/
		rng.Min = make(expr.LiteralExprList, len(el))
		rng.Max = make(expr.LiteralExprList, len(el))
		for i := range el {
			if i == len(el)-1 {
				e := el[i].(expr.LiteralExprList)
				rng.Min[i] = e[0]
				rng.Max[i] = e[1]
				continue
			}

			rng.Min[i] = el[i]
			rng.Max[i] = el[i]
		}
	}

	return rng
}

type filterNode struct {
	// associated stream node
	node stream.Operator

	// the expression of the node
	// has been broken into
	// <path> <operator> <operand>
	// Ex:    a.b[0] > 5 + 5
	// Gives:
	// - path: a.b[0]
	// - operator: scanner.GT
	// - operand: 5 + 5
	path     document.Path
	operator scanner.Token
	operand  expr.Expr
}

type filterNodes []*filterNode

// getByPath returns the first filter for the given path.
// TODO(asdine): add a rule that merges filter nodes that point to the
// same path.
func (f filterNodes) getByPath(p document.Path) *filterNode {
	for _, fn := range f {
		if fn.path.IsEqual(p) {
			return fn
		}
	}

	return nil
}

type candidate struct {
	// filter operators to remove and replace by either an index.Scan
	// or pkScan operators.
	nodes filterNodes

	// replace the table.Scan by these nodes
	replaceRootBy []stream.Operator

	// cost of the associated ranges
	rangesCost int

	// is this candidate reading from an index.
	// if false, we are reading from the table
	// primary key.
	isIndex bool
	// if it's an index, does it have a unique constraint
	isUnique bool
}

func (c *candidate) Cost() int {
	// we start with the cost of ranges
	cost := c.rangesCost

	if c.isIndex {
		cost += 20
	}
	if c.isUnique {
		cost -= 10
	}

	cost -= len(c.nodes)

	return cost
}

// operatorIsIndexCompatible returns whether the operator can be used to read from an index.
func operatorIsIndexCompatible(op expr.Operator) bool {
	switch op.Token() {
	case scanner.EQ, scanner.GT, scanner.GTE, scanner.LT, scanner.LTE, scanner.IN, scanner.BETWEEN:
		return true
	}

	return false
}

func operatorCanUseIndex(op expr.Operator) (bool, document.Path, expr.Expr) {
	lf, leftIsPath := op.LeftHand().(expr.Path)
	rf, rightIsPath := op.RightHand().(expr.Path)

	// Special case for IN operator: only left operand is valid for index usage
	// valid:   a IN [1, 2, 3]
	// invalid: 1 IN a
	// invalid: a IN (b + 1, 2)
	if op.Token() == scanner.IN {
		if leftIsPath && !rightIsPath && !exprContainsPath(op.RightHand()) {
			rh := op.RightHand()
			// The IN operator can use indexes only if the right hand side is an expression list.
			if _, ok := rh.(expr.LiteralExprList); !ok {
				return false, nil, nil
			}
			return true, document.Path(lf), rh
		}

		return false, nil, nil
	}

	// Special case for BETWEEN operator: Given this expression (x BETWEEN a AND b),
	// we can only use the index if the "x" is a path and "a" and "b" don't contain path expressions.
	if op.Token() == scanner.BETWEEN {
		bt := op.(*expr.BetweenOperator)
		x, xIsPath := bt.X.(expr.Path)
		if !xIsPath || exprContainsPath(bt.LeftHand()) || exprContainsPath(bt.RightHand()) {
			return false, nil, nil
		}

		return true, document.Path(x), expr.LiteralExprList{bt.LeftHand(), bt.RightHand()}
	}

	// path OP expr
	if leftIsPath && !rightIsPath && !exprContainsPath(op.RightHand()) {
		return true, document.Path(lf), op.RightHand()
	}

	// expr OP path
	if rightIsPath && !leftIsPath && !exprContainsPath(op.LeftHand()) {
		return true, document.Path(rf), op.LeftHand()
	}

	return false, nil, nil
}

func exprContainsPath(e expr.Expr) bool {
	var hasPath bool

	expr.Walk(e, func(e expr.Expr) bool {
		if _, ok := e.(expr.Path); ok {
			hasPath = true
			return false
		}
		return true
	})

	return hasPath
}
