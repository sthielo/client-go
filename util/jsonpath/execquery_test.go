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
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type execQueryTest struct {
	name             string
	query            string
	allowMissingKeys bool
	data             interface{}
	expectedResult   interface{}
}

func householdTestData() *household {
	return &household{
		persons: []person{
			{"Homer", "Simpson", 39},
			{"Marge", "Simpson", 36},
			{"Bart", "Simpson", 10},
			{"Lisa", "Simpson", 8},
			{"Maggie", "Simpson", 1},
		},
		// WARNING iteration order of maps IS NOT DEFINED, such that the order of results may vary!!!
		animals: map[string]animal{
			"Santa's Little Helper": dog{"brown", 7},
			"Snowball V":            cat{"black"},
		},
		address: "742 Evergreen Terrace",
		visitors: []person{
			{"Abe", "Simpson", 86},
			{"Selma", "Bouvier", 36},
		},
	}
}

var execQryTests = []execQueryTest{
	{"rootNodeIdentifierOnly", `$`, false, householdTestData(), []interface{}{householdTestData()}},
	{"nameSelector", `$.'persons'[0].'firstName'`, false, householdTestData(), []interface{}{"Homer"}},
	{"rel-filter-query existence", `$.persons[?@.'firstName'=='Homer'].'firstName'`, false, householdTestData(), []interface{}{"Homer"}},
	{"descendantNameSelector", `..'firstName'`, false, householdTestData(), []interface{}{"Homer", "Marge", "Bart", "Lisa", "Maggie", "Abe", "Selma"}},
	{"wildcardSelector", `.'visitors'.*`, false, householdTestData(), householdTestData().visitors},
	{"descendantMultipleNameSelectors", `..['firstName', 'address']`, false, []household{*householdTestData()}, []interface{}{"Homer", "Marge", "Bart", "Lisa", "Maggie", "Abe", "Selma", "742 Evergreen Terrace"}},
	{"indexSelectors", `.'persons'[1, -2].'firstName'`, false, householdTestData(), []interface{}{"Marge", "Lisa"}},
	{"arraySliceSelectors", `.'persons'[2:4].'firstName'`, false, householdTestData(), []interface{}{"Bart", "Lisa"}},
	{"arraySliceSelectorsNoStart", `.'persons'[ :3:2 ].'firstName'`, false, householdTestData(), []interface{}{"Homer", "Bart"}},
	{"arraySliceSelectorsNoEndNegSteps", `.'persons'[ 3::-2].'firstName'`, false, householdTestData(), []interface{}{"Lisa", "Marge"}},
	{"arraySliceSelectorsNegStart", `.'persons'[-2:: ].'firstName'`, false, householdTestData(), []interface{}{"Lisa", "Maggie"}},
	{"arraySliceSelectorsAllNeg", `.'persons'[ -2:-4:-1 ].'firstName'`, false, householdTestData(), []interface{}{"Lisa", "Bart"}},
	{"descendantFilter", `..[?.'age'>38].'firstName'`, true, householdTestData(), []interface{}{"Homer", "Abe"}},
	{"descendantMultipleSelectors", `..[?.'age'>38]['firstName', 'age']`, false, householdTestData(), []interface{}{"Homer", 39, "Abe", 86}},
	{"filterQrySimple", `$[?.'color' == 'brown']`, false, householdTestData().animals, []interface{}{&dog{"brown", 7}}},
	{"traversingMaps", `.."Snowball V".'color'`, false, householdTestData(), []interface{}{"black"}},
	{"filterLogicalAndComparisonOps", `..[?.'firstName'&&.'age' > 80].'age'`, false, householdTestData(), []int{86}},
	{"filterLogicalOpsSelectors", `..[?.'firstName' == 'Lisa' , ? .'color' == 'brown'].'age'`, false, householdTestData(), []int{8, 7}},
	{"filterLogicalOpsSelectors2", `..[?.'color' && .'age' > 5, ?.'firstName' == 'Lisa'].'age'`, false, householdTestData(), []int{7, 8}},
	{"fct length", "$..[?length(.'firstName')<=3].'firstName'", false, householdTestData(), []interface{}{"Abe"}},
	{"duplicates due to multiple selectors", "$..[?length(.'firstName')<=4, ?.'age'>80].'firstName'", false, householdTestData(), []interface{}{"Bart", "Lisa", "Abe", "Abe"}},
	{"registered custom fct", "$..[?length(.'firstName')<=3 && custom(.'firstName') < 0.01].'firstName'", false, householdTestData(), []interface{}{"Abe"}},
	{"absolute query", "$..[?length(.'firstName')<=3 && $.'address'].'firstName'", false, householdTestData(), []interface{}{"Abe"}},
	{"relative query", "$..[?length(.'firstName')<=3 && @.'address'].'firstName'", false, householdTestData(), []interface{}{}},
	{"not comparable is specified to be false", "$..[?length(.'firstName')<=3 && $.'address' < 1.00e-2].'firstName'", false, householdTestData(), []interface{}{}},
	{"select children by existence of non-singular nodes", "$[?@.*]", false, householdTestData(), []interface{}{householdTestData().persons, householdTestData().animals, householdTestData().visitors}},
	// todo enable/disable AllowMissingKeys
	// do tests with maps
}

var customFct QueryFunction = func(arg ...QueryResult) (QueryResult, error) {
	return &Singular{1.034e-12}, nil
}

func TestExecQuery(t *testing.T) {
	const printDebugMsgs = false
	for _, test := range execQryTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newQueryParser()
			parser.name = test.name
			err := parser.parse(test.query)
			require.Nilf(subT, err, "failed to initialize test - while parsing the query - with unexpected error: %v", err)
			es, isNil := indirect(reflect.ValueOf(test.expectedResult))
			require.False(subT, isNil, "expectedResult is nil")
			require.Contains(subT, []reflect.Kind{reflect.Array, reflect.Slice}, es.Kind(), "expectedResult must be of kind array/slice!")

			fmt.Printf("parsed  : %s\n", parser.string())

			rootDataNode := reflect.ValueOf(test.data)
			fctRegistry := newFunctionRegistry()
			err = fctRegistry.register("custom", customFct)
			require.Nil(subT, err, "error on creating fctRegistry")
			results, err := executeQuery(parser, rootDataNode, rootDataNode, false, test.allowMissingKeys, fctRegistry, printDebugMsgs)
			require.Nilf(subT, err, "failed with unexpected error: %v", err)

			fmt.Printf("expected: %s\n", test.expectedResult)
			b := bytes.Buffer{}
			printResults(&b, results, legacyFormat, "")
			fmt.Printf("result  : %s\n", b.String())

			require.Equal(subT, es.Len(), len(results.Elems), "results length not as expected")
			for i := 0; i < es.Len(); i++ {
				e := es.Index(i)
				assertEqualValue(subT, e, results.Elems[i])
			}
			println("\n")
		})
	}
}

func checkEqualArray(checkOnly bool, t *testing.T, expected reflect.Value, result reflect.Value) bool {
	e, eIsNil := indirect(expected)
	r, rIsNil := indirect(result)
	if checkOnly {
		if eIsNil || rIsNil {
			return eIsNil == rIsNil
		}
	} else {
		require.Equal(t, eIsNil, rIsNil, "value.IsNil do not match")
	}
	switch e.Kind() {
	case reflect.Array, reflect.Slice:
		if checkOnly {
			if r.Kind() != reflect.Array && r.Kind() != reflect.Slice {
				return false
			}
			if r.Len() != e.Len() {
				return false
			}
		} else {
			require.Contains(t, []reflect.Kind{reflect.Array, reflect.Slice}, r.Kind(), "expected r of kind array/slice, but got: %v", r)
			require.Equal(t, e.Len(), r.Len(), "array len not equal")
		}
		for i := 0; i < e.Len(); i++ {
			checkEqualValue(checkOnly, t, e.Index(i), r.Index(i))
		}
	default:
		require.Fail(t, "expected array/slice", "expected: %v, result: %v", expected, result)
	}
	return true
}

func assertEqualValue(t *testing.T, expected reflect.Value, result reflect.Value) bool {
	return checkEqualValue(false, t, expected, result)
}

func checkEqualValue(checkOnly bool, t *testing.T, expected reflect.Value, result reflect.Value) bool {
	e, eIsNil := indirect(expected)
	r, rIsNil := indirect(result)
	if checkOnly {
		if rIsNil != eIsNil {
			return false
		}
	} else {
		require.Equal(t, eIsNil, rIsNil, "value.IsNil do not match")
	}
	if eIsNil {
		return true
	}

	switch e.Kind() {
	case reflect.Struct:
		return checkEqualStruct(checkOnly, t, expected, result)
	case reflect.Array, reflect.Slice:
		return checkEqualArray(checkOnly, t, e, r)
	case reflect.Map:
		return checkEqualMap(checkOnly, t, e, r)
	case reflect.String:
		if checkOnly {
			if reflect.String != r.Kind() {
				return false
			}
			if e.String() != r.String() {
				return false
			}
		} else {
			require.Equal(t, reflect.String, r.Kind(), "expected kind string, but got: %v", r)
			require.Equal(t, e.String(), r.String(), "string literal do not match - expected: %s, but got: %s", e.String(), r.String())
		}
		return true
	case reflect.Int:
		if checkOnly {
			if reflect.Int != r.Kind() {
				return false
			}
			if e.Int() != r.Int() {
				return false
			}
		} else {
			require.Equal(t, reflect.Int, r.Kind(), "expected kind int, but got: %v", r)
			require.Equal(t, e.Int(), r.Int(), "int literal do not match - expected: %d, but got: %d", e.Int(), r.Int())
		}
		return true
	case reflect.Float64:
		if checkOnly {
			if reflect.Float64 != r.Kind() {
				return false
			}
			if e.Float() != r.Float() {
				return false
			}
		} else {
			require.Equal(t, reflect.Float64, r.Kind(), "expected kind float64, but got: %v", r)
			require.Equal(t, e.Float(), r.Float(), "float literal do not match - expected: %f, but got: %f", e.Float(), r.Float())
		}
		return true
	case reflect.Bool:
		if checkOnly {
			if reflect.Bool != r.Kind() {
				return false
			}
			if e.Bool() != r.Bool() {
				return false
			}
		} else {
			require.Equal(t, reflect.Bool, r.Kind(), "expected kind bool, but got: %v", r)
			require.Equal(t, e.Bool(), r.Bool(), "bool literal do not match - expected: %t, but got: %t", e.Bool(), r.Bool())
		}
		return true

	default:
		require.Failf(t, "internal error", "unkown/unconsidered value.Kind: %d", e.Kind())
		return false
	}
}

func checkEqualStruct(checkOnly bool, t *testing.T, expected reflect.Value, result reflect.Value) bool {
	e, eIsNil := indirect(expected)
	r, rIsNil := indirect(result)
	if checkOnly {
		if eIsNil != rIsNil {
			return false
		}
	} else {
		require.Equal(t, eIsNil, rIsNil, "value.IsNil do not match")
	}
	if eIsNil {
		return true
	}

	if checkOnly {
		if reflect.Struct != r.Kind() {
			return false
		}
		if e.NumField() != r.NumField() {
			return false
		}
	} else {
		require.Equal(t, reflect.Struct, r.Kind(), "expected kind struct, but got: %v", r)
		require.Equal(t, e.NumField(), r.NumField(), "struct.NumField() do not match")
	}
	for i := 0; i < e.NumField(); i++ {
		ef := e.Field(i)
		fieldName := e.Type().Field(i).Name
		rf := r.FieldByName(fieldName)
		checkEqualValue(checkOnly, t, ef, rf)
	}

	return true
}

func checkEqualMap(checkOnly bool, t *testing.T, expected reflect.Value, result reflect.Value) bool {
	e, eIsNil := indirect(expected)
	r, rIsNil := indirect(result)
	if checkOnly {
		if eIsNil != rIsNil {
			return false
		}
	} else {
		require.Equal(t, eIsNil, rIsNil, "value.IsNil do not match")
	}
	if e.IsNil() {
		return true
	}

	if checkOnly {
		if reflect.Map != r.Kind() {
			return false
		}
		if e.Len() != r.Len() {
			return false
		}
	} else {
		require.Equal(t, reflect.Map, r.Kind(), "expected kind map, but got: %v", r)
		require.Equal(t, e.Len(), r.Len(), "map.Len() do not match")
	}
	for _, k := range e.MapKeys() {
		ef := e.MapIndex(k)
		rf := r.MapIndex(k)
		checkEqualValue(checkOnly, t, ef, rf)
	}

	return true
}

type failExecQryTest struct {
	name  string
	query string
	data  interface{}
	err   string
}

var returnargFct QueryFunction = func(args ...QueryResult) (QueryResult, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("invalid nr of args to function 'length' - requires exactly ONE argument")
	}
	return args[0], nil
}

func TestFailExecQuery(t *testing.T) {
	const printDebugMsgs = false
	failTests := []failExecQryTest{
		{"undefined fct", "$[?undefined(.'persons')]", householdTestData(), "undefined fct"},
		{"fct result: singular-value required but not provided", "$.'persons'[?returnarg(@.*) <= 'abc']", householdTestData(), "expected singular-query for comparison-element"},
		{"fct result: singular-value required but not provided", "$[?returnarg($.'persons'[*]) <= 'abc']", householdTestData(), "expected singular-query for comparison-element"},
		{"fct result: singular-value required but not provided", "$[?returnarg(.'persons'[*]) <= 'abc']", householdTestData(), "expected singular-query for comparison-element"},
		// todo
		// allowMissingKeys=false
		// wrong arg type for fct (type vs value)
		// `..*.[color, age]` => invalid ...'.['...
		// not existing fct {"function_custom", "$..[?length(.firstName)<=3 && custom(.firstname) < 0.01].firstName", householdTestData(), []interface{}{"Abe"}},
	}
	for _, test := range failTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newQueryParser()
			parser.name = test.name
			err := parser.parse(test.query)
			require.Nilf(subT, err, "failed to initialize test - while parsing the query - with unexpected error: %v", err)
			fctRegistry := newFunctionRegistry()
			err = fctRegistry.register("returnarg", returnargFct)
			require.Nil(subT, err, "error on creating fctRegistry")
			_, err = executeQuery(parser, reflect.ValueOf(test.data), reflect.ValueOf(test.data), false, false, fctRegistry, printDebugMsgs)
			require.NotNilf(subT, err, "not failed as expected. expected error %v", test.err)
			println("\n")
		})
	}
}

type person struct {
	firstName string
	lastName  string
	age       int
}

type animal interface {
	getColor() string
}

type dog struct {
	color string
	age   int
}

func (a dog) getColor() string { return a.color }

type cat struct {
	color string
}

func (a cat) getColor() string { return a.color }

type household struct {
	persons  []person
	animals  map[string]animal
	address  string
	visitors []person
}
