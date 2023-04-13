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
	"github.com/stretchr/testify/require"
	"os"
	"reflect"
	"testing"
)

// from https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-12.html
// partially using maps => do not care about order of resultSet !!!

func allStoreItems() []interface{} {
	store := storeTestData().store
	result := make([]interface{}, 0, 5)
	result = append(result, store.book)
	result = append(result, store.bicycle)
	return result
}
func allStoreElementsAndMembers() []interface{} {
	store := storeTestData().store
	result := make([]interface{}, 0, 100)
	result = append(result, store)
	result = append(result, store.book)
	for _, b := range storeTestData().store.book {
		result = append(result, b)
		result = append(result, b["category"])
		result = append(result, b["author"])
		result = append(result, b["title"])
		result = append(result, b["price"])
		isbn, existsIsbn := b["isbn"]
		if existsIsbn {
			result = append(result, isbn)
		}
	}
	b := store.bicycle
	result = append(result, b)
	result = append(result, b["color"])
	result = append(result, b["price"])
	return result
}

type execQuerySpecTest struct {
	query                    string
	name                     string
	data                     interface{}
	allowMissingKeys         bool
	expectedResult           []interface{}
	orderedResultSetExpected bool
}

func nameSelectorTestData() interface{} {
	return map[string]interface{}{
		"o": map[string]interface{}{"j j": map[string]interface{}{"k.k": 3}},
		"'": map[string]int{"@": 2},
	}
}

func wildcardSelectorTestData() interface{} {
	return map[interface{}]interface{}{
		"o": map[interface{}]interface{}{"j": 1, "k": 2},
		"a": []interface{}{5, 3},
	}
}

func indexSelectorTestData() interface{} {
	return []string{"a", "b"}
}

func arraySliceSelectorTestData() interface{} {
	return []string{"a", "b", "c", "d", "e", "f", "g"}
}

func comparisonFilterSelectorTestData() interface{} {
	return map[string]interface{}{
		"a": []interface{}{3, 5, 1, 2, 4, 6, map[string]interface{}{"b": "j"}, map[string]interface{}{"b": "k"}, map[string]interface{}{"b": map[string]interface{}{}}, map[string]interface{}{"b": "kilo"}},
		"o": map[string]interface{}{"p": 1, "q": 2, "r": 3, "s": 5, "t": map[string]interface{}{"u": 6}},
		"e": "f",
	}
}
func comparTD_a() []interface{} {
	return comparisonFilterSelectorTestData().(map[string]interface{})["a"].([]interface{})
}
func comparTD_o() interface{} {
	return comparisonFilterSelectorTestData().(map[string]interface{})["o"]
}

func descendentSegmentTestData() interface{} {
	return map[string]interface{}{
		"o": map[string]interface{}{"j": 1, "k": 2},
		"a": []interface{}{5, 3, []map[string]interface{}{{"j": 4}, {"k": 6}}},
	}
}
func allDescendantValues() []interface{} {
	data := descendentSegmentTestData()
	o := data.(map[string]interface{})["o"]
	a := data.(map[string]interface{})["a"]

	result := make([]interface{}, 0, 20)

	result = append(result, o)
	result = append(result, a)
	result = append(result, 1)
	result = append(result, 2)
	result = append(result, 5)
	result = append(result, 3)
	result = append(result, []map[string]interface{}{{"j": 4}, {"k": 6}})
	result = append(result, map[string]interface{}{"j": 4})
	result = append(result, map[string]interface{}{"k": 6})
	result = append(result, 4)
	result = append(result, 6)
	return result
}

func nullTestData() interface{} {
	return map[string]interface{}{"a": nil, "b": []interface{}{nil}, "c": []interface{}{map[string]interface{}{}}, "null": 1}
}

func storeTestData() *testDataT {
	return &testDataT{storeT{
		[]map[string]interface{}{
			{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95},
			{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": 12.99},
			{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99},
			{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": 22.99},
		},
		map[string]interface{}{"color": "red", "price": 399},
	}}
}

var specTests = []execQuerySpecTest{
	// === general spec examples - unordered ... array values evaluated in order, BUT other values depend on struct field declaration order
	{"$.'store'.'book'[*].'author'", "the authors of all books in the store", storeTestData(), false, []interface{}{"Nigel Rees", "Evelyn Waugh", "Herman Melville", "J. R. R. Tolkien"}, true},
	{"$..'author'", "all authors", storeTestData(), false, []interface{}{"Nigel Rees", "Evelyn Waugh", "Herman Melville", "J. R. R. Tolkien"}, true},
	{"$.'store'.*", "all things in store, which are some books and a red bicycle", storeTestData(), false, allStoreItems(), true},
	{"$.'store'..'price'", "the prices of everything in the store", storeTestData(), false, []interface{}{8.95, 12.99, 8.99, 22.99, 399}, true},
	{"$..'book'[2]", "the third book", storeTestData(), false, []interface{}{map[string]interface{}{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99}}, true},
	{"$..'book'[-1]", "the last book in order", storeTestData(), false, []interface{}{map[string]interface{}{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": 22.99}}, true},
	{"$..'book'[0,1]", "the first two books", storeTestData(), false, []interface{}{
		map[string]interface{}{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95},
		map[string]interface{}{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": 12.99}}, true},
	{"$..'book'[:2]", "the first two books", storeTestData(), false, []interface{}{
		map[string]interface{}{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95},
		map[string]interface{}{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": 12.99},
	}, true},
	{"$..'book'[?(@.'isbn')]", "all books with an ISBN number", storeTestData(), true /*not all books have an ISBN!*/, []interface{}{
		map[string]interface{}{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99},
		map[string]interface{}{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": 22.99},
	}, true},
	{"$..'book'[?(@.'price'<10)]", "all books cheaper than 10", storeTestData(), false, []interface{}{
		map[string]interface{}{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95},
		map[string]interface{}{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99},
	}, true},
	{"$..*", "all member values and array elements contained in the input value", storeTestData(), false, allStoreElementsAndMembers(), false},

	// === name-selector examples
	{`$.o['j j']['k.k']`, "Named value in nested object", nameSelectorTestData(), false, []interface{}{3}, true},
	{`$.o["j j"]["k.k"]`, "Named value in nested object (2)", nameSelectorTestData(), false, []interface{}{3}, true},
	{`$["'"]["@"]`, "Unusual member names", nameSelectorTestData(), false, []interface{}{2}, true},

	// wildcard examples (unordered!)
	{`$[*]`, "Object values", wildcardSelectorTestData(), false, []interface{}{map[interface{}]interface{}{"j": 1, "k": 2}, []int{5, 3}}, false},
	{`$.'o'[*]`, "Object values (2)", wildcardSelectorTestData(), false, []interface{}{1, 2}, false},
	{`$.'o'[*, *]`, "Object values (3)", wildcardSelectorTestData(), false, []interface{}{1, 2, 2, 1}, false},
	{`$.'a'[*]`, "Array members", wildcardSelectorTestData(), false, []interface{}{5, 3}, false},

	// === index-selector examples
	{`$[1]`, "Element of array", indexSelectorTestData(), false, []interface{}{"b"}, true},
	{`$[-2]`, "Element of array, from the end", indexSelectorTestData(), false, []interface{}{"a"}, true},

	// === arraySlice-selector examples - ORDERED resultsets!
	{`$[1:3]`, "Slice with default step", arraySliceSelectorTestData(), false, []interface{}{"b", "c"}, true},
	{`$[5:]`, "Slice with no end index", arraySliceSelectorTestData(), false, []interface{}{"f", "g"}, true},
	{`$[1:5:2]`, "Slice with step 2", arraySliceSelectorTestData(), false, []interface{}{"b", "d"}, true},
	{`$[5:1:-2]`, "Slice with negative step", arraySliceSelectorTestData(), false, []interface{}{"f", "d"}, true},
	{`$[::-1]`, "Slice in reverse order", arraySliceSelectorTestData(), false, []interface{}{"g", "f", "e", "d", "c", "b", "a"}, true},

	// === comparison examples
	{`$.'a'[?@.'b' == 'kilo']`, "Member value comparison", comparisonFilterSelectorTestData(), true /*not all array elements have key b*/, []interface{}{map[string]string{"b": "kilo"}}, true},
	{`$.'a'[?@>3.5] `, "Array value comparison", comparisonFilterSelectorTestData(), false, []interface{}{5, 4, 6}, true},
	{`$.'a'[?@.'b']`, "Array value existence", comparisonFilterSelectorTestData(), true, []interface{}{map[string]interface{}{"b": "j"}, map[string]interface{}{"b": "k"}, map[string]interface{}{"b": map[string]interface{}{}}, map[string]interface{}{"b": "kilo"}}, true},
	{`$[?@.*]`, "Existence of non-singular queries", comparisonFilterSelectorTestData(), false, []interface{}{comparTD_a(), comparTD_o()}, false},
	{`$[?@[?@.'b']]`, "Nested filters", comparisonFilterSelectorTestData(), true, []interface{}{comparTD_a()}, true},
	{`$.'o'[?@<3, ?@<3]`, "Non-deterministic ordering", comparisonFilterSelectorTestData(), false, []interface{}{1, 2, 2, 1}, false},
	{`$.'a'[?@<2 || @.'b' == "k"]`, "Array value logical OR", comparisonFilterSelectorTestData(), true, []interface{}{1, map[string]interface{}{"b": "k"}}, true},
	{`$.'a'[?match(@.'b', "[jk]")]`, "Array value regular expression match", comparisonFilterSelectorTestData(), true, []interface{}{map[string]interface{}{"b": "j"}, map[string]interface{}{"b": "k"}}, true},
	// todo NOT YET IMPLEMENTED {`$.a[?search(@.b, "[jk]")]`, "Array value regular expression search", comparisonFilterSelectorTestData(), []interface{}{
	//	map[string]interface{}{"b": "j"}, map[string]interface{}{"b": "k"}, map[string]interface{}{"b": "kilo"},
	// }, true},
	{`$.'o'[?@>1 && @<4]`, "Object value logical AND", comparisonFilterSelectorTestData(), false, []interface{}{2, 3}, false},
	{`$.'o'[?@.'u' || @.'x']`, "Object value logical OR", comparisonFilterSelectorTestData(), true, []interface{}{map[string]interface{}{"u": 6}}, false},
	{`$.'a'[?(@.'b' == $.'x')]`, "Comparison of queries with no values", comparisonFilterSelectorTestData(), true, []interface{}{3, 5, 1, 2, 4, 6}, true},
	{`$.'a'[?(@ == @)]`, "Comparisons of primitive and of structured values", comparisonFilterSelectorTestData(), false, comparTD_a(), false},

	// === descendent examples
	{`$..'j'`, "Object values", descendentSegmentTestData(), false, []interface{}{1, 4}, false},
	{`$..[0]`, "Array values", descendentSegmentTestData(), false, []interface{}{5, map[string]interface{}{"j": 4}}, false},
	{`$..[*]`, "All values", descendentSegmentTestData(), false, allDescendantValues(), false},
	{`$..*`, "All values (2)", descendentSegmentTestData(), false, allDescendantValues(), false},
	{`$..'o'`, "Input value is visited", descendentSegmentTestData(), false, []interface{}{map[string]interface{}{"j": 1, "k": 2}}, false},
	{`$.'o'..[*, *]`, "Non-deterministic ordering", descendentSegmentTestData(), false, []interface{}{1, 2, 2, 1}, false},
	{`$.'a'..[0, 1]`, "Multiple segments", descendentSegmentTestData(), false, []interface{}{5, 3, map[string]interface{}{"j": 4}, map[string]interface{}{"k": 6}}, false},

	// === null examples
	{`$.'a'`, "Object value", nullTestData(), false, []interface{}{nil}, true},
	{`$.'a'[0]`, "null used as array", nullTestData(), false, []interface{}{}, true},
	{`$.a.d`, "null used as object", nullTestData(), false, []interface{}{}, true},
	{`$.'b'[0]`, "Array value", nullTestData(), false, []interface{}{nil}, true},
	{`$.'b'[*]`, "Array value (2)", nullTestData(), false, []interface{}{nil}, true},
	{`$.'b'[?@] `, "Existence", nullTestData(), false, []interface{}{nil}, true},
	{`$.'b'[?@==null]`, "Comparison", nullTestData(), false, []interface{}{nil}, true},
	{`$.'c'[?(@.d==null)]`, "Comparison with 'missing' value", nullTestData(), true, []interface{}{}, true},
	{`$.null`, "Not JSON null at all, just a member name string", nullTestData(), false, []interface{}{1}, true},
}

func TestSpecExecQuery(t *testing.T) {
	const printDebugMsgs = false
	for _, test := range specTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newQueryParser()
			parser.name = test.name
			err := parser.parse(test.query)
			require.Nilf(subT, err, "failed to initialize test - while parsing the query - with unexpected error: %v", err)
			fmt.Printf("\nquery: %s", parser.string())

			rootDataNode := reflect.ValueOf(test.data)
			results, err := executeQuery(parser, rootDataNode, rootDataNode, false, test.allowMissingKeys, newFunctionRegistry(), printDebugMsgs)
			require.Nilf(subT, err, "failed with unexpected error: %v", err)

			fmt.Printf("\nexpected : %v", test.expectedResult)
			fmt.Print("\nresultSet: ")
			printResults(os.Stdout, results, dbgFormat, "")

			require.True(subT, test.expectedResult == nil && results == nil || test.expectedResult != nil && results != nil, "nil results do not match")
			if test.expectedResult != nil {
				require.Equal(subT, len(test.expectedResult), len(results.Elems), "results length not as expected")
				if test.orderedResultSetExpected {
					for i := 0; i < len(test.expectedResult); i++ {
						e := test.expectedResult[i]
						assertEqualValue(subT, reflect.ValueOf(e), results.Elems[i])
					}
				} else {
					findNextExpectedInRemainingResults(subT, test.expectedResult, results.Elems)
				}
			}
			println("\n")
		})
	}
}

type storeT struct {
	book    []map[string]interface{}
	bicycle map[string]interface{}
}
type testDataT struct {
	store storeT
}
