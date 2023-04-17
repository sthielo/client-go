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
	"bytes"
	"fmt"
	"reflect"
	"strconv"
)

type evalContext interface {
	isDebugEnabled() bool
}

type qryExecContext struct {
	name              string
	dataRoot          reflect.Value
	evalExistenceOnly bool
	allowMissingKeys  bool
	remainingSegments []segment
	functions         functionRegistry
	enableDbgMsgs     bool
	curSegment        segment
	curSelector       selector
}

func (ctx qryExecContext) isDebugEnabled() bool {
	return ctx.enableDbgMsgs
}

func (ctx qryExecContext) areMissingKeysAllowed() bool {
	return ctx.allowMissingKeys || (ctx.curSegment != nil && ctx.curSegment.getType() != childSegmentType)
}

func (ctx qryExecContext) isDescending() bool {
	return ctx.curSegment != nil && ctx.curSegment.getType() == descendantSegmentType
}

func (ctx qryExecContext) duplicateAndTakeFirstSegmentOff() qryExecContext {
	return qryExecContext{ctx.name, ctx.dataRoot, ctx.evalExistenceOnly,
		ctx.allowMissingKeys, ctx.remainingSegments[1:], ctx.functions,
		ctx.enableDbgMsgs, ctx.curSegment, ctx.curSelector}
}

func emptyResultSet() *ResultSet {
	return &ResultSet{[]reflect.Value{}}
}

var dbgFormat = resultFormat{condensedJsonFormatted, "%.2f"}

func (ctx qryExecContext) dbgMsgf(msgFormat string, args ...interface{}) {
	if ctx.isDebugEnabled() {
		convertedArgs := make([]interface{}, len(args))
		for i, a := range args {
			if a == nil {
				convertedArgs[i] = "null"
			} else {
				switch a.(type) {
				case []reflect.Value:
					var b bytes.Buffer
					_, _ = fmt.Fprint(&b, "[")
					for j, aE := range a.([]reflect.Value) {
						printValue(&b, aE, dbgFormat, "")
						if j < len(a.([]reflect.Value))-1 {
							_, _ = fmt.Fprint(&b, ",")
						}
					}
					_, _ = fmt.Fprint(&b, "]")
					convertedArgs[i] = b.String()
					break
				case *ResultSet:
					rs := a.(*ResultSet)
					if rs.Elems == nil {
						convertedArgs[i] = "null"
					} else {
						var b bytes.Buffer
						_, _ = fmt.Fprint(&b, "[")
						for j, aE := range rs.Elems {
							printValue(&b, aE, dbgFormat, "")
							if j < len(rs.Elems)-1 {
								_, _ = fmt.Fprint(&b, ",")
							}
						}
						_, _ = fmt.Fprint(&b, "]")
						convertedArgs[i] = b.String()
					}
					break
				case *Singular:
					var b bytes.Buffer
					sV := a.(*Singular).Value
					printValue(&b, sV, dbgFormat, "")
					convertedArgs[i] = b.String()
					break
				case reflect.Value:
					aV, isNil := indirect(a.(reflect.Value))
					if isNil {
						convertedArgs[i] = "null"
					} else {
						var b bytes.Buffer
						printValue(&b, aV, dbgFormat, "")
						convertedArgs[i] = b.String()
					}
					break
				default:
					convertedArgs[i] = a
				}
			}
		}
		fmt.Printf(msgFormat+"\n", convertedArgs...)
	}
}

func executeQuery(p *queryParser, rootDataNode reflect.Value, currDataNode reflect.Value, evalExistenceOnly bool, allowMissingKeys bool, fcts functionRegistry, dbgMsgs bool) (*ResultSet, error) {
	qryRoot := currDataNode
	// handle JSONPath nodeIdentifiers
	switch p.root.nodeIdentifierSymbol {
	case rootNodeSymbol:
		qryRoot = rootDataNode
		break
	case currentNodeSymbol:
		// already set
		break
	default:
		panic(fmt.Sprintf("internal error - unknown nodeIdentifierSymbol: %d", p.root.nodeIdentifierSymbol))
	}

	ctx := &qryExecContext{p.name, rootDataNode, evalExistenceOnly, allowMissingKeys, p.root.segments, fcts, dbgMsgs, nil, nil}
	ctx.dbgMsgf("entering query: '%s' on '%s'", p.string(), qryRoot)
	results, err := findResults(*ctx, qryRoot)
	if err != nil {
		ctx.dbgMsgf("query failed (evalExistenceOnly=%t): '%s' on '%s' => err=%v", evalExistenceOnly, p.string(), qryRoot, err)
		return nil, err
	}
	ctx.dbgMsgf("query result (evalExistenceOnly=%t): '%s' on '%s' => results to %s", evalExistenceOnly, p.string(), qryRoot, results)
	return results, err
}

func findResults(ctx qryExecContext, curNode reflect.Value) (*ResultSet, error) {
	// end of JSONPath segments
	if len(ctx.remainingSegments) <= 0 {
		ctx.dbgMsgf("found a result - no segments left on node: %s", curNode)
		return &ResultSet{[]reflect.Value{curNode}}, nil
	}

	n, isNil := indirect(curNode)
	if isNil {
		// no more segments my match anything
		return emptyResultSet(), nil
	}

	results := make([]reflect.Value, 0, 5)
	ctx.curSegment = ctx.remainingSegments[0]
	ctx.dbgMsgf("entering segment: '%s'", ctx.curSegment.string())
	for _, sel := range ctx.curSegment.getSelectors() {
		ctx.curSelector = sel
		ctx.dbgMsgf("entering selector: '%s'", ctx.curSelector.string())
		nodeResults, err := selectChildrenAndFindResultsForThem(ctx.duplicateAndTakeFirstSegmentOff(), n)
		ctx.curSelector = nil
		if err != nil {
			return nil, err
		}
		if nodeResults != nil && len(nodeResults.Elems) > 0 {
			if ctx.evalExistenceOnly {
				return nodeResults, nil
			}
			results = append(results, nodeResults.Elems...)
		}
	}
	return &ResultSet{results}, nil
}

func selectChildrenAndFindResultsForThem(ctx qryExecContext, curNode reflect.Value) (*ResultSet, error) {
	n, isNil := indirect(curNode)
	if isNil {
		return emptyResultSet(), nil
	}

	switch ctx.curSelector.(type) {
	case *wildcardSelector:
		return selectAllChildrenAndFindResultsForThem(ctx, n)
	case *nameSelector:
		return selectChildByNameAndFindResultsForThem(ctx, n)
	case *indexSelector:
		return selectChildByIndexAndFindResultsForThem(ctx, n)
	case *arraySliceSelector:
		return selectChildrenBySliceAndFindResultsForThem(ctx, n)
	case *filterSelector:
		return selectChildrenByFilterAndFindResultsForThem(ctx, n)
	default:
		panic(fmt.Sprintf("internal error - unknown selectorType: %#v", ctx.curSelector))
	}
}

func selectAllChildrenAndFindResultsForThem(ctx qryExecContext, curNode reflect.Value) (*ResultSet, error) {
	return walkChildren(ctx, curNode, false, func(_ reflect.Kind, child reflect.Value, _ interface{}, _ int) (bool, error) {
		return true, nil
	})
}

func selectChildByNameAndFindResultsForThem(ctx qryExecContext, curNode reflect.Value) (*ResultSet, error) {
	curNameSelector := ctx.curSelector.(*nameSelector)
	if !ctx.areMissingKeysAllowed() {
		n, isNil := indirect(curNode)
		if isNil {
			return nil, ExecutionError{ctx.name, fmt.Sprintf("Missing key (value is nil): %s", curNameSelector.name)}
		}
		switch n.Kind() {
		case reflect.Struct, reflect.Map:
			break
		default:
			return nil, ExecutionError{ctx.name, fmt.Sprintf("Missing key (object not of named-values type - kind: %d): %s", n.Kind(), ctx.curSelector.(*nameSelector).name)}
		}
	}
	// applies only to Map/Struct
	missingKey := true
	childResults, err := walkChildren(ctx, curNode, false, func(k reflect.Kind, child reflect.Value, fieldKey interface{}, _ int) (bool, error) {
		switch k {
		case reflect.Map:
			// map keys can be of any type - only strings supported here
			switch fieldKey.(type) {
			case string:
				selected := fieldKey.(string) == curNameSelector.name
				missingKey = missingKey && !selected
				return selected, nil
			case reflect.Value:
				fV, isNil := indirect(fieldKey.(reflect.Value))
				if !isNil && fV.Kind() == reflect.String {
					selected := fV.String() == curNameSelector.name
					missingKey = missingKey && !selected
					return selected, nil
				}
			}
			ctx.dbgMsgf("encountered map key type other than string when trying to apply a name-selector, which only supports strings: map key type: %T %#“", fieldKey)
			return false, nil
		case reflect.Struct:
			// fieldNames are always strings
			selected := fieldKey.(string) == curNameSelector.name
			missingKey = missingKey && !selected
			return selected, nil
		default:
			return false, nil
		}
	})
	if !ctx.areMissingKeysAllowed() && missingKey {
		return nil, ExecutionError{ctx.name, fmt.Sprintf("Missing key (key does not exist): %s", curNameSelector.name)}
	}
	return childResults, err
}

func selectChildByIndexAndFindResultsForThem(ctx qryExecContext, curNode reflect.Value) (*ResultSet, error) {
	n, isNil := indirect(curNode)
	if isNil {
		return emptyResultSet(), nil
	}

	curIndexSelector := ctx.curSelector.(*indexSelector)

	selIndex := curIndexSelector.index
	switch n.Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		if selIndex < 0 {
			selIndex = n.Len() + selIndex
		}
		if !ctx.areMissingKeysAllowed() && (selIndex >= n.Len() || selIndex < 0) {
			return nil, ExecutionError{ctx.name, fmt.Sprintf("missing key: index-selector > length. index: %d", selIndex)}
		}
	default:
		if !ctx.areMissingKeysAllowed() {
			return nil, ExecutionError{ctx.name, fmt.Sprintf("missing key: index-selector for non-array/slice/string object. object.Kind: %d", curNode.Kind())}
		}
	}

	// applies only for array/slice
	return walkChildren(ctx, curNode, false, func(k reflect.Kind, child reflect.Value, _ interface{}, index int) (bool, error) {
		switch k {
		case reflect.Array, reflect.Slice:
			return index == selIndex, nil
		default:
			return false, nil
		}
	})
}

func selectChildrenBySliceAndFindResultsForThem(ctx qryExecContext, curNode reflect.Value) (*ResultSet, error) {
	_, isNil := indirect(curNode)
	if isNil {
		return emptyResultSet(), nil
	}

	curSliceSelector := ctx.curSelector.(*arraySliceSelector)

	arrLength := -1
	switch curNode.Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		arrLength = curNode.Len()
		sliceStep := curSliceSelector.step
		if sliceStep == 0 {
			// according to spec: no result
			return nil, nil
		}
		sliceStart := 0       // default
		sliceEnd := arrLength // default
		step := 1

		// negative steps
		if sliceStep < 0 {
			// switch defaults
			if arrLength >= 1 {
				sliceStart = arrLength - 1 // default for neg step
			}
			sliceEnd = -1 // default for neg step
			step = -1
		}

		if curSliceSelector.start.isDefined {
			// negative start: count from end
			if curSliceSelector.start.intValue < 0 {
				if arrLength+curSliceSelector.start.intValue > 0 {
					sliceStart = arrLength + curSliceSelector.start.intValue
				} else {
					sliceStart = 0
				}
			} else {
				sliceStart = curSliceSelector.start.intValue
			}
		}
		if curSliceSelector.end.isDefined {
			// negative start: count from end
			if curSliceSelector.end.intValue < 0 {
				sliceEnd = arrLength + curSliceSelector.end.intValue
			} else {
				sliceEnd = curSliceSelector.end.intValue
			}
		}

		if step > 0 && sliceEnd-sliceStart < 0 || step < 0 && sliceEnd-sliceStart > 0 {
			ctx.dbgMsgf("empty slice encountered: %s became %d:%d:%d for arrayLen=%d", curSliceSelector.string(), sliceStart, sliceEnd, step, arrLength)
		}

		nextSliceIndex := sliceStart

		// applies only for array/slice
		return walkChildren(ctx, curNode, step < 0, func(k reflect.Kind, child reflect.Value, _ interface{}, index int) (bool, error) {
			switch k {
			case reflect.Array, reflect.Slice, reflect.String:
				if index == nextSliceIndex && (step > 0 && nextSliceIndex < sliceEnd || step < 0 && nextSliceIndex > sliceEnd) {
					nextSliceIndex += sliceStep
					return true, nil
				}
				return false, nil
			default:
				panic("internal error - only array/slice/string expected")
			}
		})

	default:
		if !ctx.areMissingKeysAllowed() {
			return nil, ExecutionError{ctx.name, fmt.Sprintf("missing key: slice-selector on non-array/slice/string object: %s", curSliceSelector.string())}
		}
		return walkChildren(ctx, curNode, false, func(k reflect.Kind, _ reflect.Value, _ interface{}, _ int) (bool, error) {
			switch k {
			case reflect.Array, reflect.Slice, reflect.String:
				panic("internal error - NOT expected: array/slice/string")
			default:
				return false, nil
			}
		})
	}
}

func doWithSelected(ctx qryExecContext, selected reflect.Value) (*ResultSet, error) {
	n, isNil := indirect(selected)
	selectedNodeResults := make([]reflect.Value, 0, 1)
	if len(ctx.remainingSegments) <= 0 {
		ctx.dbgMsgf("found a result - selected child and no segments left: %s", n)
		if ctx.evalExistenceOnly {
			return &ResultSet{[]reflect.Value{selected}}, nil
		}
		selectedNodeResults = append(selectedNodeResults, selected)
	} else {
		if isNil {
			return emptyResultSet(), nil
		}

		childResults, err := findResults(ctx, n)
		if err != nil {
			return nil, err
		}
		if childResults != nil && len(childResults.Elems) > 0 {
			if ctx.evalExistenceOnly {
				return childResults, nil
			}
			selectedNodeResults = append(selectedNodeResults, childResults.Elems...)
		}
	}

	if !isNil && ctx.isDescending() && typeHasChildren(n.Kind()) {
		ctx.dbgMsgf("descending with same selector '%s' to children of: %s", ctx.curSelector.string(), n)
		// same segment => same ctx, same selFct
		descendantResults, err := selectChildrenAndFindResultsForThem(ctx, n)
		if err != nil {
			return nil, err
		}
		if descendantResults != nil && len(descendantResults.Elems) > 0 {
			if ctx.evalExistenceOnly {
				return descendantResults, nil
			}
			selectedNodeResults = append(selectedNodeResults, descendantResults.Elems...)
		}
	}
	return &ResultSet{selectedNodeResults}, nil
}

func doWithNotSelected(ctx qryExecContext, notSelected reflect.Value) (*ResultSet, error) {
	n, isNil := indirect(notSelected)
	if isNil {
		return emptyResultSet(), nil
	}
	if ctx.isDescending() && typeHasChildren(n.Kind()) {
		ctx.dbgMsgf("descending with same selector '%s' to children of currNode: %s", ctx.curSelector.string(), n)
		// same segment => same ctx, same selFct
		return selectChildrenAndFindResultsForThem(ctx, n)
	}
	return emptyResultSet(), nil
}

type selectorEvalFct func(parentKind reflect.Kind, node reflect.Value, fieldKey interface{}, index int) (bool, error)

func walkChildren(ctx qryExecContext, curNode reflect.Value, reverseOrder bool, selFct selectorEvalFct) (*ResultSet, error) {
	currNodeVal, currNodeIsNil := indirect(curNode)
	if currNodeIsNil {
		return emptyResultSet(), nil
	}

	results := make([]reflect.Value, 0, 10)

	var withChildDo = func(parentKind reflect.Kind, child reflect.Value, fieldKey interface{}, index int) error {
		isSelected, err := selFct(parentKind, child, fieldKey, index)
		if err != nil {
			return err
		}
		var childResults *ResultSet
		if isSelected {
			childResults, err = doWithSelected(ctx, child)
		} else {
			childResults, err = doWithNotSelected(ctx, child)
		}
		if err != nil {
			return err
		}
		if childResults != nil && len(childResults.Elems) > 0 {
			results = append(results, childResults.Elems...)
		}
		return nil
	}

	switch currNodeVal.Kind() {
	case reflect.Struct:
		for i := 0; i < currNodeVal.NumField(); i++ {
			child := currNodeVal.Field(i)
			fieldName := currNodeVal.Type().Field(i).Name
			err := withChildDo(currNodeVal.Kind(), child, fieldName, i)
			if err != nil {
				return nil, err
			}
			if ctx.evalExistenceOnly && len(results) > 0 {
				return &ResultSet{results}, nil
			}
		}
		break

	case reflect.Map:
		for _, key := range currNodeVal.MapKeys() {
			child := currNodeVal.MapIndex(key)
			err := withChildDo(currNodeVal.Kind(), child, key, -1)
			if err != nil {
				return nil, err
			}
			if ctx.evalExistenceOnly && len(results) > 0 {
				return &ResultSet{results}, nil
			}
		}
		break

	case reflect.Array, reflect.Slice:
		start := 0
		end := currNodeVal.Len()
		step := 1
		if reverseOrder {
			start = currNodeVal.Len() - 1
			end = -1
			step = -1

		}
		for i := start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
			child := currNodeVal.Index(i)
			err := withChildDo(currNodeVal.Kind(), child, "", i)
			if err != nil {
				return nil, err
			}
			if ctx.evalExistenceOnly && len(results) > 0 {
				return &ResultSet{results}, nil
			}
		}
		break

	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
		// these types do not have children to traverse
		break

	default:
		panic(fmt.Sprintf("internal error - unsupported kind of child"))
	}
	return &ResultSet{results}, nil
}

func selectChildrenByFilterAndFindResultsForThem(ctx qryExecContext, curNode reflect.Value) (*ResultSet, error) {
	curFilterSelector := ctx.curSelector.(*filterSelector)
	return walkChildren(ctx, curNode, false, func(_ reflect.Kind, child reflect.Value, _ interface{}, _ int) (bool, error) {
		expr, err := evalBoolSingularOrExistenceExpr(ctx, child, curFilterSelector.expr)
		if err != nil || expr == nil || expr.Value == nil {
			return false, err
		}
		return expr.Value.(bool), err
	})
}

// evalExpr top level filterExpr must evaluate to bool, interpreted to whether the current node is a match
func evalExpr(ctx qryExecContext, curNode reflect.Value, expr filterExpr) (QueryResult, error) {
	var dbgMsgEvalResult = func(r QueryResult, e error) (QueryResult, error) {
		if e != nil {
			ctx.dbgMsgf("filterExpr '%s' failed: err=%s - curNode: %s", expr.string(), e.Error(), curNode)
		} else {
			ctx.dbgMsgf("filterExpr '%s' evaluated to: %s - curNode: %s", expr.string(), r, curNode)
		}
		return r, e
	}
	ctx.dbgMsgf("entering filterExpr: %s for node curNode=%s", expr.string(), curNode)
	switch expr.getType() {
	case logicalExprType:
		return dbgMsgEvalResult(evalLogicalExpr(ctx, curNode, expr.(*logicalExpr)))

	case compareExprType:
		return dbgMsgEvalResult(evalCompareExpr(ctx, curNode, expr.(*compareExpr)))

	case filterQryType:
		return dbgMsgEvalResult(evalFilterQryExpr(ctx, curNode, expr.(*filterQry)))

	case functionExprType:
		return dbgMsgEvalResult(evalFunctionExpr(ctx, curNode, expr.(*functionExpr)))

	case parenExprType:
		return dbgMsgEvalResult(evalExpr(ctx, curNode, expr.(*parenExpr).inner))

	case stringLiteralType:
		return dbgMsgEvalResult(&Singular{expr.(*stringLiteral).val}, nil)

	case boolLiteralType:
		return dbgMsgEvalResult(&Singular{expr.(*boolLiteral).val}, nil)

	case nullLiteralType:
		return dbgMsgEvalResult(&Singular{nil}, nil)

	case intLiteralType:
		return dbgMsgEvalResult(&Singular{expr.(*intLiteral).val}, nil)

	case floatLiteralType:
		return dbgMsgEvalResult(&Singular{expr.(*floatLiteral).val}, nil)

	default:
		panic(fmt.Sprintf("internal error - unknown filterExprType: %d", expr.getType()))
	}
}

func evalBoolSingularOrExistenceExpr(ctx qryExecContext, curNode reflect.Value, expr filterExpr) (*Singular, error) {
	exprResult, err := evalExpr(ctx, curNode, expr)
	if err != nil {
		return nil, err
	}
	if exprResult == nil {
		// anything invalid is interpreted as false upon evaluation
		return nil, nil
	}
	switch exprResult.(type) {
	case *ResultSet:
		return &Singular{len(exprResult.(*ResultSet).Elems) > 0}, nil
	case *Singular:
		v := exprResult.(*Singular).Value
		if v == nil {
			return nil, ExecutionError{ctx.name, fmt.Sprintf("invalid exprResult 'nil' to be used within logical expression: %#v", expr)}
		}
		switch v.(type) {
		case bool:
			return exprResult.(*Singular), nil
		default:
			return nil, ExecutionError{ctx.name, fmt.Sprintf("invalid exprResult type to be used within logical expression: %#v", expr)}
		}
	default:
		panic(fmt.Sprintf("unknown result type: %#v", exprResult))
	}
}

func evalLogicalExpr(ctx qryExecContext, curNode reflect.Value, expr *logicalExpr) (*Singular, error) {
	leftExprVal, err := evalBoolSingularOrExistenceExpr(ctx, curNode, expr.left)
	if err != nil {
		return nil, err
	}
	left := leftExprVal.Value.(bool)
	if expr.logicalOp == notOp {
		return &Singular{!left}, nil
	}
	if !left && expr.logicalOp == andOp {
		return &Singular{false}, nil
	}
	if left && expr.logicalOp == orOp {
		return &Singular{true}, nil
	}
	rightExprVal, err := evalBoolSingularOrExistenceExpr(ctx, curNode, expr.right)
	if err != nil {
		return nil, err
	}
	return &Singular{rightExprVal.Value.(bool)}, nil
}

func evalSingularExpr(ctx qryExecContext, curNode reflect.Value, expr filterExpr) (*Singular, error) {
	exprResult, err := evalExpr(ctx, curNode, expr)
	if err != nil {
		return nil, err
	}
	if exprResult == nil {
		return nil, nil
	}
	switch exprResult.(type) {
	case *ResultSet:
		rSet := exprResult.(*ResultSet)
		l := len(rSet.Elems)
		if l > 1 {
			return nil, fmt.Errorf("internal error - resultSet with multiple elements presented to comparison-expression: %s", expr.string())
		}
		if l <= 0 {
			return nil, nil
		}
		singularResult, isNil := indirect(rSet.Elems[0])
		if isNil {
			return &Singular{nil}, nil
		}
		switch singularResult.Kind() {
		case reflect.Bool:
			return &Singular{singularResult.Bool()}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return &Singular{singularResult.Int()}, nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return &Singular{singularResult.Uint()}, nil
		case reflect.Float32, reflect.Float64:
			return &Singular{singularResult.Float()}, nil
		case reflect.String:
			return &Singular{singularResult.String()}, nil
		case reflect.Array, reflect.Slice, reflect.Map, reflect.Struct:
			return &Singular{singularResult}, nil
		default:
			panic(fmt.Sprintf("unsupported singular value type: kind=%d of %#v", singularResult.Kind(), singularResult))
		}
	case *Singular:
		return exprResult.(*Singular), nil
	default:
		panic(fmt.Sprintf("unknown result type: %#v", exprResult))
	}
}

func evalCompareExpr(ctx qryExecContext, curNode reflect.Value, expr *compareExpr) (*Singular, error) {
	left, err := evalSingularExpr(ctx, curNode, expr.left)
	if err != nil {
		return nil, err
	}

	right, err := evalSingularExpr(ctx, curNode, expr.right)
	if err != nil {
		return nil, err
	}

	b := compareValues(ctx, left, right, expr.compareOp)
	return &Singular{b}, nil
}

func compareValues(ctx qryExecContext, l *Singular, r *Singular, op comparisonOpTypeEnum) bool {
	if l == nil || r == nil {
		// either side did not evaluate to a proper value
		return l == nil && r == nil
	}
	if l.Value == nil || r.Value == nil {
		// either side did evaluate to JSON 'null' value
		return l.Value == nil && r.Value == nil
	}
	lV, ok := l.Value.(reflect.Value)
	if !ok {
		lV = reflect.ValueOf(l.Value)
	}
	rV, ok := r.Value.(reflect.Value)
	if !ok {
		rV = reflect.ValueOf(r.Value)
	}
	return compareRValues(ctx, lV, rV, op)
}

func compareRValues(ctx qryExecContext, l reflect.Value, r reflect.Value, op comparisonOpTypeEnum) bool {
	lV, lIsNil := indirect(l)
	rV, rIsNil := indirect(r)
	if lIsNil || rIsNil {
		return lIsNil == rIsNil
	}
	switch lV.Kind() {
	case reflect.Bool:
		return compareBoolTo(ctx, lV.Bool(), rV, op)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return compareIntTo(ctx, lV.Int(), rV, op)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return compareUintTo(ctx, lV.Uint(), rV, op)
	case reflect.Float32, reflect.Float64:
		return compareFloatTo(ctx, lV.Float(), rV, op)
	case reflect.String:
		return compareStringTo(ctx, lV.String(), rV, op)
	case reflect.Map, reflect.Struct:
		return compareNamedValuesTo(ctx, lV, rV, op)
	case reflect.Array, reflect.Slice:
		return compareArrayTo(ctx, lV, rV, op)
	default:
		panic(fmt.Sprintf("internal error - unsupport value.Kind: %#v", lV))
	}
}

func compareBoolTo(ctx qryExecContext, l bool, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch r.Kind() {
	case reflect.Bool:
		return compareBool(ctx, l, r.Bool(), op)
	case reflect.String:
		b, err := strconv.ParseBool(r.String())
		if err != nil {
			return false // cannot convert string '%s' to bool for comparison: %v", r.string()
		}
		return compareBool(ctx, l, b, op)
	default:
		ctx.dbgMsgf("right hand sight value cannot be compared with bool: %s", r)
		return false
	}
}

func compareBool(ctx qryExecContext, l bool, r bool, op comparisonOpTypeEnum) bool {
	switch op {
	case eqOp:
		return l == r
	case neOp:
		return l != r
	case ltOp, gtOp, leOp, geOp:
		ctx.dbgMsgf("op cannot be used for bool values: '%s'", op)
		return false // according to spec
	default:
		panic(fmt.Sprintf("internal error - unknown compare-operator: %s", op))
	}
}

func compareIntTo(ctx qryExecContext, l int64, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch r.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return compareInt(l, r.Int(), op)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if l < 0 {
			switch op {
			case eqOp:
				return false
			case neOp:
				return true
			case ltOp, leOp:
				return true
			case gtOp, geOp:
				return false
			default:
				panic(fmt.Sprintf("internal error - compare-operator: %s", op))
			}
		}
		return compareUint(uint64(l), r.Uint(), op)
	case reflect.Float32, reflect.Float64:
		return compareFloat(float64(l), r.Float(), op)
	case reflect.String:
		ri, err := strconv.ParseInt(r.String(), 10, 64)
		if err != nil {
			return false // cannot convert string '%s' to int for comparison: %v", r.string()
		}
		return compareInt(l, ri, op)
	case reflect.Map, reflect.Struct, reflect.Array, reflect.Slice, reflect.Bool:
		ctx.dbgMsgf("invalid right hand sight comparison type: Int - (kind=%d) %s", r.Kind(), r)
		return false
	default:
		panic(fmt.Sprintf("internal error - unsupprted value.Kind: %#v", r.Kind()))
	}
}

func compareInt(l int64, r int64, op comparisonOpTypeEnum) bool {
	switch op {
	case eqOp:
		return l == r
	case neOp:
		return l != r
	case ltOp:
		return l < r
	case gtOp:
		return l > r
	case leOp:
		return l <= r
	case geOp:
		return l >= r
	default:
		panic(fmt.Sprintf("internal error - compare-operator: %s", op))
	}
}

func compareUint(l uint64, r uint64, op comparisonOpTypeEnum) bool {
	switch op {
	case eqOp:
		return l == r
	case neOp:
		return l != r
	case ltOp:
		return l < r
	case gtOp:
		return l > r
	case leOp:
		return l <= r
	case geOp:
		return l >= r
	default:
		panic(fmt.Sprintf("internal error - compare-operator: %s", op))
	}
}

func compareString(l string, r string, op comparisonOpTypeEnum) bool {
	switch op {
	case eqOp:
		return l == r
	case neOp:
		return l != r
	case ltOp:
		return l < r
	case gtOp:
		return l > r
	case leOp:
		return l <= r
	case geOp:
		return l >= r
	default:
		panic(fmt.Sprintf("internal error - compare-operator: %s", op))
	}
}

func compareFloat(l float64, r float64, op comparisonOpTypeEnum) bool {
	switch op {
	case eqOp:
		return l == r
	case neOp:
		return l != r
	case ltOp:
		return l < r
	case gtOp:
		return l > r
	case leOp:
		return l <= r
	case geOp:
		return l >= r
	default:
		panic(fmt.Sprintf("internal error - compare-operator: %s", op))
	}
}

func compareUintTo(ctx qryExecContext, l uint64, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch r.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if r.Int() < 0 {
			switch op {
			case eqOp:
				return false
			case neOp:
				return true
			case ltOp, leOp:
				return false
			case gtOp, geOp:
				return true
			default:
				panic(fmt.Sprintf("internal error - compare-operator: %s", op))
			}
		}
		return compareUint(l, uint64(r.Int()), op)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return compareUint(l, r.Uint(), op)
	case reflect.Float32, reflect.Float64:
		return compareFloat(float64(l), r.Float(), op)
	case reflect.String:
		ru, err := strconv.ParseUint(r.String(), 10, 64)
		if err != nil {
			return false // cannot convert string '%s' to uint for comparison: %v", r.string()
		}
		return compareUint(l, ru, op)
	case reflect.Map, reflect.Struct, reflect.Array, reflect.Slice, reflect.Bool:
		ctx.dbgMsgf("invalid right hand sight comparison type: Uint - (kind=%d) %s", r.Kind(), r)
		return false
	default:
		panic(fmt.Sprintf("internal error - unsupprted value.Kind: %#v", r.Kind()))
	}
}

func compareFloatTo(ctx qryExecContext, l float64, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch r.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return compareFloat(l, float64(r.Int()), op)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return compareFloat(l, float64(r.Uint()), op)
	case reflect.Float32, reflect.Float64:
		return compareFloat(l, r.Float(), op)
	case reflect.String:
		rf, err := strconv.ParseFloat(r.String(), 64)
		if err != nil {
			return false // "cannot convert string '%s' to float for comparison: %v", r.string()
		}
		return compareFloat(l, rf, op)
	case reflect.Map, reflect.Struct, reflect.Array, reflect.Slice, reflect.Bool:
		ctx.dbgMsgf("invalid right hand sight comparison type: Float - (kind=%d) %s", r.Kind(), r)
		return false
	default:
		panic(fmt.Sprintf("internal error - unsupprted value.Kind: %#v", r.Kind()))
	}
}

func compareArrayTo(ctx qryExecContext, l reflect.Value, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch r.Kind() {
	case reflect.Array, reflect.Slice:
		return compareArray(ctx, l, r, op)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String, reflect.Map, reflect.Struct, reflect.Bool:
		ctx.dbgMsgf("invalid right hand sight comparison type: Array - (kind=%d) %s", r.Kind(), r)
		return false
	default:
		panic(fmt.Sprintf("internal error - unsupprted value.Kind for: %d", r.Kind()))
	}
}

func compareArray(ctx qryExecContext, l reflect.Value, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch op {
	case eqOp, neOp:
		if l.Len() != r.Len() {
			return op != eqOp
		}
		for i := 0; i < l.Len(); i++ {
			le := l.Index(i)
			re := r.Index(i)
			compareElemResult := compareRValues(ctx, le, re, op)
			if !compareElemResult {
				return op != eqOp
			}
		}
		return op == eqOp
	case ltOp, gtOp, leOp, geOp:
		return false // according to spec
	default:
		panic(fmt.Sprintf("internal error - unknown compare-operator: %s", op))
	}
}

func compareStringTo(ctx qryExecContext, l string, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch r.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		li, err := strconv.ParseInt(l, 10, 64)
		if err != nil {
			return false // cannot convert string to int for comparison
		}
		return compareInt(li, r.Int(), op)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		lu, err := strconv.ParseUint(l, 10, 64)
		if err != nil {
			return false // cannot convert string to uint for comparison
		}
		return compareUint(lu, r.Uint(), op)
	case reflect.Float32, reflect.Float64:
		lf, err := strconv.ParseFloat(l, 64)
		if err != nil {
			return false // cannot convert string to float for comparison
		}
		return compareFloat(lf, r.Float(), op)
	case reflect.Bool:
		lb, err := strconv.ParseBool(l)
		if err != nil {
			return false // cannot convert string to bool for comparison
		}
		return compareBool(ctx, lb, r.Bool(), op)
	case reflect.String:
		return compareString(l, r.String(), op)
	case reflect.Map, reflect.Struct, reflect.Array, reflect.Slice:
		ctx.dbgMsgf("invalid right hand sight comparison type: String - (kind=%d) %s", r.Kind(), r)
		return false
	default:
		panic(fmt.Sprintf("internal error - unsupprted value.Kind: %#v", r.Kind()))
	}
}

func compareNamedValuesTo(ctx qryExecContext, l reflect.Value, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch r.Kind() {
	case reflect.Map, reflect.Struct:
		return compareNamedValues(ctx, l, r, op)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String, reflect.Array, reflect.Slice, reflect.Bool:
		ctx.dbgMsgf("invalid right hand sight comparison type: NamedValues (%s) - %s", l, r)
		return false
	default:
		panic(fmt.Sprintf("internal error - unsupprted value.Kind: %#v", r.Kind()))
	}
}

func compareNamedValues(ctx qryExecContext, l reflect.Value, r reflect.Value, op comparisonOpTypeEnum) bool {
	switch op {
	case eqOp, neOp:
		if l.Len() != r.Len() {
			return op != eqOp
		}
		switch l.Kind() {
		case reflect.Struct:
			for i := 0; i < l.NumField(); i++ {
				fieldName := l.Type().Field(i).Name // fieldKey is a string!
				leftFieldVal := l.Field(i)
				var rightFieldVal reflect.Value
				switch r.Kind() {
				case reflect.Struct:
					rightFieldVal = r.FieldByName(fieldName)
					break
				case reflect.Map:
					rightFieldVal = r.MapIndex(reflect.ValueOf(fieldName))
					break
				default:
					panic(fmt.Sprintf("internal error - unknown named-value container: %#v", r))
				}
				if rightFieldVal.IsZero() { // field for name not found
					return op != eqOp
				}
				fieldResult := compareRValues(ctx, leftFieldVal, rightFieldVal, op)
				if !fieldResult {
					return op != eqOp
				}
			}
			break
		case reflect.Map:
			for _, fieldKey := range l.MapKeys() { // fieldKey can be of any type
				leftFieldVal := l.MapIndex(fieldKey)
				var rightFieldVal reflect.Value
				switch r.Kind() {
				case reflect.Struct:
					if fieldKey.Kind() == reflect.String {
						// structs can only have string type fieldNames
						rightFieldVal = r.FieldByName(fieldKey.String())
					} else {
						return op != eqOp
					}
					break
				case reflect.Map:
					rightFieldVal = r.MapIndex(fieldKey)
					break
				default:
					panic(fmt.Sprintf("internal error - unknown named-value container: %#v", r))
				}
				if rightFieldVal.IsZero() { // field for name not found
					return op != eqOp
				}
				fieldResult := compareRValues(ctx, leftFieldVal, rightFieldVal, op)
				if !fieldResult {
					return op != eqOp
				}
			}
			break
		default:
			panic(fmt.Sprintf("internal error - unknown named-value container: %#v", l))
		}
		return op == eqOp

	case ltOp, gtOp, leOp, geOp:
		return false // according to spec

	default:
		panic(fmt.Sprintf("internal error - unknown compare-operator: %s", op))
	}
}

func evalFunctionExpr(ctx qryExecContext, curNode reflect.Value, expr *functionExpr) (QueryResult, error) {
	fct, exists := ctx.functions[expr.fct]
	if !exists {
		return nil, ExecutionError{ctx.name, fmt.Sprintf("fct '%s' does not exist", expr.fct)}
	}
	argResults := make([]QueryResult, len(expr.args))
	for i, a := range expr.args {
		aR, err := evalExpr(ctx, curNode, a)
		if err != nil {
			return nil, err
		}
		argResults[i] = aR
	}
	result, err := fct(argResults...)
	if err != nil {
		return nil, err
	}
	return result, err
}

func evalFilterQryExpr(ctx qryExecContext, curNode reflect.Value, expr *filterQry) (QueryResult, error) {
	allowMissingKeyInFilterQry := ctx.allowMissingKeys
	if !allowMissingKeyInFilterQry && expr.parser.root.nodeIdentifierSymbol == currentNodeSymbol {
		// relative-filter-queries on descendant-segments will always encounter unknown fields.
		allowMissingKeyInFilterQry = ctx.areMissingKeysAllowed()
	}
	qryResult, err := executeQuery(expr.parser, ctx.dataRoot, curNode, expr.evalExistenceOnly, allowMissingKeyInFilterQry, ctx.functions, ctx.enableDbgMsgs)
	if err != nil {
		return nil, err
	}
	return qryResult, nil
}

func typeHasChildren(k reflect.Kind) bool {
	switch k {
	case reflect.Array, reflect.Slice, reflect.Struct, reflect.Map:
		return true
	default:
		return false
	}
}
