package jsonpath

import (
	"fmt"
	"strconv"
)

type templateElem interface {
	tmplElemInheritanceLimiter()
	string() string
}

type textTemplateElem struct {
	text string
}

func (e textTemplateElem) tmplElemInheritanceLimiter() {}
func (e textTemplateElem) string() string {
	return fmt.Sprintf("{%s}", strconv.Quote(e.text))
}

type jsonpathTemplateElem struct {
	qryParser *queryParser
}

func (e jsonpathTemplateElem) tmplElemInheritanceLimiter() {}
func (e jsonpathTemplateElem) string() string {
	return fmt.Sprintf("{%s}", e.qryParser.string())
}

type rangeTemplateElem struct {
	qryParser *queryParser
	elems     []templateElem
}

func (e rangeTemplateElem) tmplElemInheritanceLimiter() {}
func (e rangeTemplateElem) string() string {
	result := fmt.Sprintf("{range %s}", e.qryParser.string())
	for _, r := range e.elems {
		result += r.string()
	}
	result += "{end}"
	return result
}

type nodeIdentifierSymbol rune

const (
	rootNodeSymbol    nodeIdentifierSymbol = '$'
	currentNodeSymbol nodeIdentifierSymbol = '@'
)

type nodeIdentifier struct {
	nodeIdentifierSymbol nodeIdentifierSymbol
	segments             []segment
}

func (i *nodeIdentifier) appendSegment(s segment) {
	i.segments = append(i.segments, s)
}
func (i nodeIdentifier) string() string {
	result := string(i.nodeIdentifierSymbol)
	for _, s := range i.segments {
		result += s.string()
	}
	return result
}

type segmentTypeEnum uint

const (
	childSegmentType segmentTypeEnum = iota + 10
	descendantSegmentType
)

type segment interface {
	getType() segmentTypeEnum
	getSelectors() []selector
	append(selector)
	string() string
}

type segmentImpl struct {
	segmentType segmentTypeEnum
	selectors   []selector
}

func (s segmentImpl) getType() segmentTypeEnum {
	return s.segmentType
}
func (s segmentImpl) getSelectors() []selector {
	return s.selectors
}
func (s *segmentImpl) append(selector selector) {
	s.selectors = append(s.selectors, selector)
}
func (s segmentImpl) string() string {
	result := ""
	if s.segmentType == descendantSegmentType {
		result += ".."
	}
	result += "["
	for i, sel := range s.selectors {
		result += sel.string()
		if i < len(s.selectors)-1 {
			result += ","
		}
	}
	result += "]"
	return result
}

type selector interface {
	string() string
}

type wildcardSelector struct {
}

func (_ wildcardSelector) string() string {
	return "*"
}

type nameSelector struct {
	name string
}

func (s nameSelector) string() string {
	return strconv.Quote(s.name)
}

type indexSelector struct {
	index int
}

func (s indexSelector) string() string {
	return fmt.Sprintf("%d", s.index)
}

type optionalInt struct {
	isDefined bool
	intValue  int
}

var undefinedOptionalInt = optionalInt{false, 0}

type arraySliceSelector struct {
	start optionalInt
	end   optionalInt
	step  int
}

func (a arraySliceSelector) string() string {
	result := ""
	if a.start.isDefined {
		result += strconv.Itoa(a.start.intValue)
	}
	result += ":"
	if a.end.isDefined {
		result += strconv.Itoa(a.end.intValue)
	}
	result += ":" + strconv.Itoa(a.step)
	return result
}

type filterSelector struct {
	expr filterExpr
}

func newFilterSelector(expr filterExpr) *filterSelector {
	fe := expr
	if fe.getType() == parenExprType {
		fe = expr.(*parenExpr).inner
	}
	switch fe.getType() {
	case filterQryType:
		fe.(*filterQry).evalExistenceOnly = true
		break
	case logicalExprType, compareExprType, functionExprType, stringLiteralType, intLiteralType,
		floatLiteralType, boolLiteralType, nullLiteralType:
		break
	case parenExprType:
		panic("internal error - found nested parenExpr, which should not be possible")
	default:
		panic(fmt.Sprintf("unknown filterExpr type: %d in %#v", fe.getType(), fe))
	}
	return &filterSelector{fe}
}

func (s filterSelector) string() string {
	return fmt.Sprintf("?%s", s.expr.string())
}

type filterExprTypeEnum uint

const (
	logicalExprType  filterExprTypeEnum = iota + 30 // bool
	compareExprType                                 // bool
	filterQryType                                   // bool, +optionally: string, int, float
	functionExprType                                // bool, string, int, float
	parenExprType                                   // bool
	stringLiteralType
	intLiteralType
	floatLiteralType
	boolLiteralType
	nullLiteralType
)

type filterExpr interface {
	getType() filterExprTypeEnum
	string() string
	isSingular() bool
}
type filterQry struct {
	// will be set/adjusted when used within logical-expr or comparison-expr
	evalExistenceOnly bool

	parser *queryParser
}

func (_ filterQry) getType() filterExprTypeEnum { return filterQryType }
func (fq filterQry) string() string {
	return fmt.Sprintf("{evalExistenceOnly=%t}%s", fq.evalExistenceOnly, fq.parser.string())
}
func (fq filterQry) isSingular() bool { return fq.parser.isSingular }

type functionExpr struct {
	fct  string
	args []filterExpr
}

func newFunctionExpr(fctName string, args []filterExpr) *functionExpr {
	optimzedArgs := make([]filterExpr, len(args))
	for i, a := range args {
		switch a.getType() {
		case parenExprType:
			optimzedArgs[i] = a.(*parenExpr).inner
			break
		case filterQryType, compareExprType, logicalExprType, functionExprType, stringLiteralType, intLiteralType,
			floatLiteralType, boolLiteralType, nullLiteralType:
			optimzedArgs[i] = a
			break
		default:
			panic(fmt.Sprintf("internal error - unknown filterExprType: %d of %#v", a.getType(), a))
		}
	}
	return &functionExpr{fctName, optimzedArgs}
}

func (_ functionExpr) getType() filterExprTypeEnum { return functionExprType }
func (fe functionExpr) string() string {
	argsResult := ""
	for i, a := range fe.args {
		argsResult += a.string()
		if i < len(fe.args)-1 {
			argsResult += ","
		}
	}
	return fmt.Sprintf("%s(%s)", fe.fct, argsResult)
}

func (fe functionExpr) isSingular() bool {
	// could be - depends on function. e.g. length/count are returning singular values, but others might not
	return false
}

type logicalOpTypeEnum string

const ( // logical Ops
	andOp logicalOpTypeEnum = "&&"
	orOp                    = "||"
	notOp                   = "!"
)

type logicalExpr struct {
	left, right filterExpr
	logicalOp   logicalOpTypeEnum
}

func newLogicalExpr(left filterExpr, right filterExpr, op logicalOpTypeEnum) *logicalExpr {
	// set existence check to optimize execution performance
	switch left.getType() {
	case filterQryType:
		left.(*filterQry).evalExistenceOnly = true
		break
	case parenExprType:
		if left.(*parenExpr).inner.getType() == filterQryType {
			left.(*parenExpr).inner.(*filterQry).evalExistenceOnly = true
		}
	case compareExprType, logicalExprType, functionExprType, stringLiteralType, intLiteralType, floatLiteralType,
		boolLiteralType, nullLiteralType:
		break
	default:
		panic(fmt.Sprintf("internal error - unknown filterExprType: %d of %#v", left.getType(), left))
	}

	if op != notOp {
		switch right.getType() {
		case filterQryType:
			right.(*filterQry).evalExistenceOnly = true
		case parenExprType:
			if right.(*parenExpr).inner.getType() == filterQryType {
				right.(*parenExpr).inner.(*filterQry).evalExistenceOnly = true
			}
		case compareExprType, logicalExprType, functionExprType, stringLiteralType, intLiteralType, floatLiteralType,
			boolLiteralType, nullLiteralType:
			break
		default:
			panic(fmt.Sprintf("internal error - unknown filterExprType: %d of %#v", right.getType(), right))
		}
	}

	return &logicalExpr{left, right, op}
}

func (_ logicalExpr) getType() filterExprTypeEnum { return logicalExprType }
func (le logicalExpr) string() string {
	if le.logicalOp == notOp {
		return fmt.Sprintf("%s%s", le.logicalOp, le.left.string())
	} else {
		return fmt.Sprintf("%s%s%s", le.left.string(), le.logicalOp, le.right.string())
	}
}
func (_ logicalExpr) isSingular() bool { return true }

type comparisonOpTypeEnum string

const ( // compare Ops
	eqOp comparisonOpTypeEnum = "=="
	ltOp                      = "<"
	gtOp                      = ">"
	leOp                      = "<="
	geOp                      = ">="
	neOp                      = "!="
)

type compareExpr struct {
	left, right filterExpr
	compareOp   comparisonOpTypeEnum
}

func newCompareExpr(left filterExpr, right filterExpr, op comparisonOpTypeEnum) (*compareExpr, error) {
	// assert singular-arg as comparison elements
	switch left.getType() {
	case filterQryType:
		// todo only allow singular-queries (spec) WITHOUT descending-segments? => many examples and backward-compatibility will suffer
		// or throw error upon execution if query actually returns multiple elements
		left.(*filterQry).evalExistenceOnly = false
		break
	case parenExprType:
		// a pure query within parenthesis used for comparison is valid => it is interpreted for existence and
		// therefore delivers a singular bool value
		if left.(*parenExpr).inner.getType() == filterQryType {
			left.(*parenExpr).inner.(*filterQry).evalExistenceOnly = true
		}
	case compareExprType:
		return nil, fmt.Errorf("cascading comparisons not allowed in: %#v", left)
	case logicalExprType:
		panic("internal error - should not be possible, as comparison-op must have priority over logical-op in")
	case functionExprType: // can return singular values depending of the fct used, but we only know upon getting the fct's result during execution!
		break
	case stringLiteralType, intLiteralType, floatLiteralType, boolLiteralType, nullLiteralType: // singular vales by definition
		break
	default:
		panic(fmt.Sprintf("internal error - unknown filterExprType: %d of %#v", left.getType(), left))
	}

	switch right.getType() {
	case filterQryType:
		// todo only allow singular-queries (spec) WITHOUT descending-segments? => many examples and backward-compatibility will suffer
		// or throw error upon execution if query actually returns multiple elements
		right.(*filterQry).evalExistenceOnly = false
	case parenExprType:
		// a pure query within parenthesis used for comparison is valid => it is interpreted for existence and
		// therefore delivers a singular bool value
		if right.(*parenExpr).inner.getType() == filterQryType {
			right.(*parenExpr).inner.(*filterQry).evalExistenceOnly = true
		}
	case compareExprType:
		return nil, fmt.Errorf("cascading comparisons not allowed in: %#v", right)
	case logicalExprType:
		panic("internal error - should not be possible, as comparison-op must have priority over logical-op in")
	case functionExprType: // can return singular values depending of the fct used, but we only know upon getting the fct's result during execution!
		break
	case stringLiteralType, intLiteralType, floatLiteralType, boolLiteralType, nullLiteralType: // singular vales by definition
		break
	default:
		panic(fmt.Sprintf("internal error - unknown filterExprType: %d of %#v", right.getType(), right))
	}

	return &compareExpr{left, right, op}, nil
}

func (_ compareExpr) getType() filterExprTypeEnum { return compareExprType }
func (ce compareExpr) string() string {
	return fmt.Sprintf("%s%s%s", ce.left.string(), ce.compareOp, ce.right.string())
}
func (_ compareExpr) isSingular() bool { return true }

type parenExpr struct {
	inner filterExpr
}

func newParenExpr(expr filterExpr) filterExpr {
	// a little execution optimization no nested parenExpr or unnecessary wrapped in parenthesis arg !
	switch expr.getType() {
	case parenExprType, functionExprType, stringLiteralType, intLiteralType, floatLiteralType, boolLiteralType, nullLiteralType:
		return expr
	case filterQryType:
		// an inner query might deliver a result set, but will be interpreted for existence and therefore the
		// arg in parenthesis delivers a singular bool value
		return &parenExpr{expr}
	case compareExprType, logicalExprType:
		return &parenExpr{expr}
	default:
		panic(fmt.Sprintf("internal error - unknown filterExprType: %d of %#v", expr.getType(), expr))
	}
}

func (_ parenExpr) getType() filterExprTypeEnum { return parenExprType }
func (pe parenExpr) string() string {
	return fmt.Sprintf("(%s)", pe.inner.string())
}
func (pe parenExpr) isSingular() bool { return true }

type stringLiteral struct {
	val string
}

func (_ stringLiteral) getType() filterExprTypeEnum { return stringLiteralType }
func (sl stringLiteral) string() string {
	return fmt.Sprintf("%s", strconv.Quote(sl.val))
}
func (_ stringLiteral) isSingular() bool { return true }

type intLiteral struct {
	val int64
}

func (_ intLiteral) getType() filterExprTypeEnum { return intLiteralType }
func (il intLiteral) string() string {
	return fmt.Sprintf("%d", il.val)
}
func (_ intLiteral) isSingular() bool { return true }

type floatLiteral struct {
	val float64
}

func (_ floatLiteral) getType() filterExprTypeEnum { return floatLiteralType }
func (fl floatLiteral) string() string {
	return fmt.Sprintf("%e", fl.val)
}
func (_ floatLiteral) isSingular() bool { return true }

type boolLiteral struct {
	val bool
}

func (_ boolLiteral) getType() filterExprTypeEnum { return boolLiteralType }
func (bl boolLiteral) string() string {
	return fmt.Sprintf("%t", bl.val)
}
func (_ boolLiteral) isSingular() bool { return true }

type nullLiteral struct {
}

func (_ nullLiteral) getType() filterExprTypeEnum { return nullLiteralType }
func (nl nullLiteral) string() string {
	return "null"
}
func (_ nullLiteral) isSingular() bool { return true }
