/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jsonpath

import (
	"fmt"
	"strconv"
	"unicode"
)

type queryParser struct {
	name string
	root *nodeIdentifier

	// spec: comparisons only allow singular-values as elements. os if using jsonpath queries as such, they must be
	// singular-queries, which are defined to only contain (direct child?) segments with a single index- or name-selector
	isSingular bool

	// internal usage
	p *innerParser
}

// newQueryParser parses the given JSONPath query and returns a JSONPath-Query qryParser.
// If an error is encountered, parsing stops and an empty
// queryParser is returned with the error
func newQueryParser() *queryParser {
	return &queryParser{
		name:       "JSONPathQry",
		p:          nil,
		isSingular: false,
	}
}

func (p *queryParser) parse(query string) error {
	if len(query) <= 0 {
		return SyntaxError{p.name, "invalid query - empty", query, 0}
	}
	p.p = &innerParser{query, 0, 0, 0, 0}
	return p.parseNodeIdentifier(true)
}

func (p *queryParser) string() string {
	return fmt.Sprintf("{name=%s,singular=%t}%s", p.name, p.isSingular, p.root.string())
}

// parseInnerQuery internal constructor fct, that allows to set additional properties
// and handing over of already existing innerParser
// absDefaultContext allows graceful handling of qry lacking the '$'/'@' node identifiers at the beginning
// evalExistenceOnly can mark queries used within filters to optimize by aborting on first result found
// innerParser can be passed in from [Template]templateParser
func parseInnerQuery(name string, absDefaultContext bool, innerParser *innerParser) (*queryParser, error) {
	p := &queryParser{
		name:       name,
		p:          innerParser,
		isSingular: false,
	}
	err := p.parseNodeIdentifier(absDefaultContext)
	if err != nil {
		return nil, err
	}
	isSingularQry := true
Loop:
	for _, s := range p.root.segments {
		if s.getType() == descendantSegmentType || len(s.getSelectors()) != 1 {
			isSingularQry = false
			break Loop
		}
		switch s.getSelectors()[0].(type) {
		case *nameSelector, *indexSelector:
			break
		default:
			isSingularQry = false
			break Loop
		}
	}
	p.isSingular = isSingularQry
	return p, err
}

func (p *queryParser) parseNodeIdentifier(absDefaultContext bool) error {
	r := p.p.peekConsumingAllWhitespaces()
	switch r {
	case rune(rootNodeSymbol):
		p.p.consumeNext()
		p.root = &nodeIdentifier{rootNodeSymbol, make([]segment, 0, 10)}
		break
	case rune(currentNodeSymbol):
		p.p.consumeNext()
		p.root = &nodeIdentifier{currentNodeSymbol, make([]segment, 0, 10)}
		break
	default:
		if absDefaultContext {
			p.root = &nodeIdentifier{rootNodeSymbol, make([]segment, 0, 10)}
		} else {
			p.root = &nodeIdentifier{currentNodeSymbol, make([]segment, 0, 10)}
		}
	}
	return p.parseSegment()
}

// parseSegment parses a JSONPath segment
func (p *queryParser) parseSegment() error {
	for {
		switch r := p.p.peekConsumingAllWhitespaces(); {
		case r == '[':
			segment := &segmentImpl{childSegmentType, make([]selector, 0, 1)}
			err := p.parseSquareSelectors(segment)
			if err != nil {
				return err
			}
			p.root.appendSegment(segment)
			break
		case r == '.':
			err := p.parseDot()
			if err != nil {
				return err
			}
			break
		default:
			// no valid segment start detected
			return nil
		}
	}
}

// parseDot parses a JSONPath segment starting with '.'
func (p *queryParser) parseDot() error {
	p.p.consumeNext() // ASSERT: must be '.'
	segType := childSegmentType
	if p.p.peek() == '.' { // 2nd dot => descendant segment
		p.p.consumeNext()
		segType = descendantSegmentType
	}
	switch r := p.p.peek(); {
	case r == '[':
		// spec: square brackets ONLY after '..' (NOT after single '.')
		if segType != descendantSegmentType {
			return SyntaxError{p.name, "unexpected '[' (hint: use EITHER '.' or '[]' notation for child-segments. only descendent-segments may use '..' followed by '[]'-selector)", p.p.input, p.p.pos}
		}
		segment := &segmentImpl{segType, make([]selector, 0, 1)}
		err := p.parseSquareSelectors(segment)
		if err != nil {
			return err
		}
		p.root.appendSegment(segment)
		return nil
	case r == '*':
		p.p.consumeNext()
		p.root.appendSegment(&segmentImpl{segType, []selector{&wildcardSelector{}}})
		return nil
	case r == '"' || r == '\'':
		s, err := p.p.parseQuote()
		if err != nil {
			return SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
		p.root.appendSegment(&segmentImpl{segType, []selector{&nameSelector{s}}})
		return nil
	case isAlphaNumeric(r):
		// todo allow unquoted name-selectors? alpha-numeric only? or even some additional chars (non-quotes, non-whitespaces, none of '.[*')?
		// https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-12.html#name-name-selector : requires quotes
		// https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-12.html#tbl-filter : examples allow no-quotes for 'simple' (=alphanumeric? just letters, digits and '_'?) names
		name, err := p.parseAlphaNumeric()
		if err != nil {
			return err
		}
		p.root.appendSegment(&segmentImpl{segType, []selector{&nameSelector{name}}})
		return nil
	default: // no valid segment start detected
		return SyntaxError{p.name, "no valid selector found after 1st '.'", p.p.input, p.p.pos}
	}
}

// parseSquareSelectors parses a list of selectors wrapped in square brackets
func (p *queryParser) parseSquareSelectors(segment segment) error {
	_, err := p.p.unwrapByDelimiters('[', ']', func() (interface{}, error) {
	SelectorList:
		for {
			err := p.parseSelector(segment)
			if err != nil {
				return nil, err
			}
			r := p.p.peekConsumingAllWhitespaces()
		NextSelector:
			switch r {
			case ',':
				p.p.consumeNext()
				break NextSelector
			default:
				break SelectorList
			}
		}
		return nil, nil
	}, true)

	if err != nil {
		switch err.(type) {
		case SyntaxError:
			return err
		default:
			return SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
	}
	return nil
}

// parseSelector parses a single selector
func (p *queryParser) parseSelector(segment segment) error {
	switch r := p.p.peekConsumingAllWhitespaces(); {
	case r == '"' || r == '\'':
		s, err := p.p.parseQuote()
		if err != nil {
			return SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
		segment.append(&nameSelector{s})
		return nil
	case r == '*':
		p.p.consumeNext()
		segment.append(&wildcardSelector{})
		return nil
	case r == '+' || r == '-' || unicode.IsDigit(r) || r == ':': // indexSelector | arraySliceSelector
		return p.parseIndexOrArraySliceSelector(segment)
	case r == '?':
		p.p.consumeNext()
		expr, err := p.parseFilterExpressions()
		if err != nil {
			return err
		}
		segment.append(newFilterSelector(expr))
		return nil
	case isAlphaNumeric(r):
		// todo allow unquoted name-selectors? alpha-numeric only? or even some additional chars (non-quotes, non-whitespaces, none of '.[*')?
		// https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-12.html#name-name-selector : requires quotes
		// https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-12.html#tbl-filter : examples allow no-quotes for 'simple' (=alphanumeric? just letters, digits and '_'?) names
		name, err := p.parseAlphaNumeric()
		if err != nil {
			return err
		}
		segment.append(&nameSelector{name})
		return nil
	default:
		return SyntaxError{p.name, "no valid selector detected", p.p.input, p.p.pos}
	}
}

// parseIndexOrArraySliceSelector parses a single selector starting with a number, which can be an index-selector or
// an array-slice-selector
func (p *queryParser) parseIndexOrArraySliceSelector(segment segment) error {
	switch p.p.peekConsumingAllWhitespaces() {
	case ':': // arraySliceSelector with empty start
		p.p.consumeNext()
		start, end, step, err := p.parseArraySliceValues(undefinedOptionalInt)
		if err != nil {
			return err
		}
		segment.append(&arraySliceSelector{start, end, step})
		break
	case '-', '+', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		i, err := p.p.parseInteger()
		if err != nil {
			return SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
		switch p.p.peekConsumingAllWhitespaces() {
		case ':':
			p.p.consumeNext()
			start, end, step, err := p.parseArraySliceValues(optionalInt{true, i})
			if err != nil {
				return err
			}
			segment.append(&arraySliceSelector{start, end, step})
			break
		default:
			segment.append(&indexSelector{i})
		}
		break
	default:
		return SyntaxError{p.name, "invalid index/arraySlice selector", p.p.input, p.p.pos}
	}
	return nil
}

// parseFilterExpressions parses a 'chain' of filter-expressions that might be combined/chained using compare and
// logical operators
func (p *queryParser) parseFilterExpressions() (filterExpr, error) {
	var expr filterExpr
	var err error
	r := p.p.peekConsumingAllWhitespaces()
	switch r {
	case '!': // prefixed logical not-arg
		p.p.consumeNext()
		expr, err = p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
		expr = newLogicalExpr(expr, nil, notOp)
		break
	default:
		expr, err = p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
	}
	return p.parseFilterOpAndRightExpr(expr)
}

// parseFilterOpAndRightExpr given an already parsed expression, this fct checks and parses a subsequently
// 'chained' expression using any of the compare or logical operators.
// It considers '&&' to "attract stronger" than '||'. Only singular filter-queries (name- and index-segment only) are
// allowed in combination with compare-operators.
func (p *queryParser) parseFilterOpAndRightExpr(leftExpr filterExpr) (filterExpr, error) {
	var expr filterExpr
	var op comparisonOpTypeEnum
	r := p.p.peekConsumingAllWhitespaces()
	switch r {
	case '<', '=', '>', '!':
		p.p.next()
		r2 := p.p.peek()
		switch r2 {
		case '=':
			p.p.next()
		default:
			if r == '=' {
				return nil, SyntaxError{p.name, "invalid compare-operator: %s (hint: use '==' for eq. valid ops: ==, <=, <, >, >=, !, !=)", p.p.input, p.p.pos}
			}
		}
		op = comparisonOpTypeEnum(p.p.consume())
		// only get ONE next filter expression and use it for comparison with priority!
		rightExpr, err := p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
		if leftExpr.getType() == logicalExprType {
			panic(fmt.Sprintf("qryParser is not supposed to be greedy after a logical expression, but first evaluate to the right. only comparisons must greedy having higher stickyness than logical ops."))
		}
		expr, err = newCompareExpr(leftExpr, rightExpr, op)
		if err != nil {
			return nil, SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
		break

	case '&', '|':
		var op logicalOpTypeEnum
		p.p.next()
		r2 := p.p.peek()
		switch r2 {
		case r:
			p.p.next()
			op = logicalOpTypeEnum(p.p.consume())
		default:
			return nil, SyntaxError{p.name, fmt.Sprintf("invalid logical operator. (Hint: Did you mean %s?)", string(r)+string(r2)), p.p.input, p.p.pos}
		}
		// evaluate right hand sight first, as logical ops have less priority than others
		rightExpr, err := p.parseFilterExpressions()
		if err != nil {
			return nil, err
		}
		// consider priority of logical operators: '&&' before '||' - need to ensure correct evaluation order when AND 'after' OR
		if op == andOp && leftExpr.getType() == logicalExprType && leftExpr.(*logicalExpr).logicalOp == orOp {
			prevOrExpr := leftExpr.(*logicalExpr)
			expr = newLogicalExpr(prevOrExpr.left, newLogicalExpr(prevOrExpr.right, rightExpr, op), orOp)
		} else if op == andOp && rightExpr.getType() == logicalExprType && rightExpr.(*logicalExpr).logicalOp == orOp {
			followingOrExpr := rightExpr.(*logicalExpr)
			expr = newLogicalExpr(newLogicalExpr(leftExpr, followingOrExpr.left, op), followingOrExpr.right, orOp)
		} else {
			expr = newLogicalExpr(leftExpr, rightExpr, op)
		}
		break

	default: // assuming end of filter expression
		return leftExpr, nil
	}
	// check for subsequent operator
	return p.parseFilterOpAndRightExpr(expr)
}

// parseFilterExpr parses a single filter-expression
func (p *queryParser) parseFilterExpr() (filterExpr, error) {
	switch p.p.peekConsumingAllWhitespaces() {
	case '@', '$', '.', '[':
		return p.parseFilterQryExpr()
	case '(':
		exprs, err := p.parseParenthesisExpr(false)
		if err != nil {
			return nil, err
		}
		if exprs == nil || len(exprs) != 1 {
			return nil, SyntaxError{p.name, "only a single expression can be contained within expression parenthesis", p.p.input, p.p.pos}
		}
		return exprs[0], err
	default:
		break
	}
	// expect text based expression: literal or fct
	return p.parseTextExpr()
}

// parseParenthesisExpr parses a optionally 'chained' filter expressions within parenthesis
func (p *queryParser) parseParenthesisExpr(allowMultiple bool) ([]filterExpr, error) {
	result, err := p.p.unwrapByDelimiters('(', ')', func() (interface{}, error) {
		results := make([]filterExpr, 0, 2)
		for {
			expr, err := p.parseFilterExpressions()
			if err != nil {
				return nil, err
			}
			if allowMultiple {
				results = append(results, expr)
			} else {
				results = append(results, newParenExpr(expr))
				return results, nil
			}
			r := p.p.peekConsumingAllWhitespaces()
			switch r {
			case ',': // parse another one
				p.p.consumeNext()
				break
			case ')':
				return results, nil
			default:
				return nil, SyntaxError{p.name, "invalid syntax - ',' or ')' expected", p.p.input, p.p.pos}
			}
		}
	}, true)
	if err != nil {
		switch err.(type) {
		case SyntaxError:
			return nil, err
		default:
			return nil, SyntaxError{p.name, err.Error() + "(hint: text literals in query filters must be quoted, otherwise they are interpreted as alphanumeric fct names. Custom functions can be registered.))", p.p.input, p.p.pos}
		}
	}
	return result.([]filterExpr), nil
}

// parseTextExpr parses a literal- or a fct-filter-expression
func (p *queryParser) parseTextExpr() (filterExpr, error) {
	switch r := p.p.peekConsumingAllWhitespaces(); {
	case r == '"' || r == '\'':
		s, err := p.p.parseQuote()
		if err != nil {
			return nil, SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
		return &stringLiteral{s}, nil

	case unicode.IsDigit(r) || r == '-' || r == '+':
		return p.parseNumberLiteral()

	case isAlphaNumeric(r):
		s, err := p.parseAlphaNumeric()
		if err != nil {
			return nil, err
		}
		switch s {
		case "true", "false": // spec declares only these lower case identifiers to be valid!
			v, _ := strconv.ParseBool(s)
			return &boolLiteral{v}, nil
		case "null": /// todo shall we allow "nil"? not valid according to specs!
			return &nullLiteral{}, nil
		default:
			exprs, err := p.parseParenthesisExpr(true)
			if err != nil {
				return nil, err
			}
			return newFunctionExpr(s, exprs), nil
		}
	}
	return nil, SyntaxError{p.name, "unexpected char", p.p.input, p.p.pos}
}

func (p *queryParser) parseNumberLiteral() (filterExpr, error) {
	switch p.p.peek() {
	case '-', '+':
		p.p.next()
	}
	decimalSep := false
	expSep := false
Loop:
	for {
	NextChar:
		switch r := p.p.peek(); {
		case r == '.' && !decimalSep:
			p.p.next()
			decimalSep = true
			break NextChar
		case r == 'e' && !expSep:
			p.p.next()
			expSep = true
			switch p.p.peek() {
			case '-', '+':
				p.p.next()
			}
			break NextChar
		case unicode.IsDigit(r):
			p.p.next()
			break NextChar
		default: // not a number char
			break Loop
		}
	}
	s := p.p.consume()
	if decimalSep || expSep {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, SyntaxError{p.name, fmt.Sprintf("invalid float: %v", err), p.p.input, p.p.pos}
		}
		return &floatLiteral{f}, nil
	} else {
		i, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, SyntaxError{p.name, fmt.Sprintf("invalid integer: %v", err), p.p.input, p.p.pos}
		}
		return &intLiteral{i}, nil
	}
}

// parseFilterQryExpr parses a singular-query expression
func (p *queryParser) parseFilterQryExpr() (filterExpr, error) {
	p.p.subQryCnt++
	filterQryParser, err := parseInnerQuery(fmt.Sprintf("filterQry-%d", p.p.subQryCnt-1), false, p.p)
	if err != nil {
		return nil, err
	}
	return &filterQry{false, filterQryParser}, nil
}

// parseAlphaNumeric parses an alphaNumeric identifier (incl allowing '_')
func (p *queryParser) parseAlphaNumeric() (string, error) {
Loop:
	for {
		switch r := p.p.peek(); {
		case r == '\\':
			return "", SyntaxError{p.name, "escaping not allowed in unquoted name-selectors", p.p.input, p.p.pos}
		case isAlphaNumeric(r):
			p.p.next()
			break
		default:
			break Loop
		}
	}
	return p.p.consume(), nil
}

func (p *queryParser) parseArraySliceValues(start optionalInt) (optionalInt, optionalInt, int, error) {
	var end optionalInt
	r := p.p.peekConsumingAllWhitespaces()
	switch r {
	case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		endVal, err := p.p.parseInteger()
		if err != nil {
			return undefinedOptionalInt, undefinedOptionalInt, 0, SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
		end = optionalInt{true, endVal}
		break
	case ':': // empty value for end
		end = undefinedOptionalInt
		break
	default: // seems only to be: '<start>:' - not even having a 2nd [optional] colon or a value for <end>
		return start, end, 1, nil
	}

	r = p.p.peekConsumingAllWhitespaces()
	if r != ':' { // no 2nd colon
		return start, end, 1, nil
	}
	p.p.consumeNext()

	r = p.p.peekConsumingAllWhitespaces()
	switch r {
	case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		stepVal, err := p.p.parseInteger()
		if err != nil {
			return undefinedOptionalInt, undefinedOptionalInt, 0, SyntaxError{p.name, err.Error(), p.p.input, p.p.pos}
		}
		return start, end, stepVal, nil
	}
	return start, end, 1, nil
}
