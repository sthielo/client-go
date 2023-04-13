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
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type jsonpathTest struct {
	name                string
	template            string
	allowMissingKeys    bool
	input               interface{}
	expect              string
	expectOrderedResult bool
	expectError         bool
}

func testJSONPath(tests []jsonpathTest, t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(subT *testing.T) {
			fmt.Printf("\ntest name           : %s", test.name)
			fmt.Printf("\ntest template       : %s", test.template)
			j, err := NewJSONPath(test.name, test.template)
			require.Nil(subT, err, "parsing of '%s' failed with: %q", test.template, err)
			require.NotNil(subT, j.parser, "parser is nil. failed parsing?")
			fmt.Printf("\nparse result        : %s", j.parser.string())
			require.Truef(subT, test.expectError || err == nil, "parseNodeIdentifier %s error %v", test.template, err)
			j.AllowMissingKeys(test.allowMissingKeys)
			_ = j.SetFloatFormat("%.2f")
			var buf bytes.Buffer
			err = j.Execute(&buf, test.input)
			fmt.Printf("\ndata                : %s", test.input)
			if test.expectError {
				fmt.Print("\nexpected error")
			} else {
				fmt.Printf("\nexpected            : %s", test.expect)
			}
			require.Truef(subT, !test.expectError || err != nil, "expected execTmpl '%s' error, got %q", test.template, buf)
			require.Truef(subT, test.expectError || err == nil, "executeQuery error %#v", err)
			out := buf.String()
			fmt.Printf("\nexecution result    : %s\n", out)
			requireExpectedString(subT, test.expect, out, test.expectOrderedResult)
			println("\n")
		})
	}
}

// testJSONPathSortOutput test cases related to map, the results may print in random order
func testJSONPathSortOutput(tests []jsonpathTest, t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(subT *testing.T) {
			j, err := NewJSONPath(test.name, test.template)
			require.Nilf(subT, err, "parseNodeIdentifier %s error %v", test.template, err)
			j.AllowMissingKeys(test.allowMissingKeys)
			buf := new(bytes.Buffer)
			err = j.Execute(buf, test.input)
			require.Nilf(subT, err, "executeQuery error %#v", err)
			out := buf.String()
			//since map is visited in random order, we need to sort the results.
			sortedOut := strings.Fields(out)
			sort.Strings(sortedOut)
			sortedExpect := strings.Fields(test.expect)
			sort.Strings(sortedExpect)
			if !reflect.DeepEqual(sortedOut, sortedExpect) {
				require.Fail(subT, "expect to get '%s', got '%s''", test.expect, out)
			}
			println("\n")
		})
	}
}

func testFailJSONPath(tests []jsonpathTest, t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(subT *testing.T) {
			j, err := NewJSONPath(test.name, test.template)
			j.AllowMissingKeys(test.allowMissingKeys)
			require.Nilf(subT, err, "parsing of '%s' failed with: %v", test.template, err)
			buf := new(bytes.Buffer)
			err = j.Execute(buf, test.input)
			require.NotNil(subT, err, "expect to get error %q, got result %q", test.expect, buf.String())
			println("\n")
		})
	}
}

func TestTypesInput(t *testing.T) {
	types := map[string]interface{}{
		"bools":      []bool{true, false, true, false},
		"integers":   []int{1, 2, 3, 4},
		"floats":     []float64{1.0, 2.2, 3.3, 4.0},
		"strings":    []string{"one", "two", "three", "four"},
		"interfaces": []interface{}{true, "one", 1, 1.1},
		"maps": []map[string]interface{}{
			{"name": "one", "value": 1},
			{"name": "two", "value": 2.02},
			{"name": "three", "value": 3.03},
			{"name": "four", "value": 4.04},
		},
		"structs": []struct {
			Name  string      `json:"name"`
			Value interface{} `json:"value"`
			Type  string      `json:"type"`
		}{
			{Name: "one", Value: 1, Type: "integer"},
			{Name: "two", Value: 2.002, Type: "float"},
			{Name: "three", Value: 3, Type: "integer"},
			{Name: "four", Value: 4.004, Type: "float"},
		},
	}

	sliceTests := []jsonpathTest{
		// boolean slice tests
		{"boolSlice", `{ .bools }`, false, types, `[true,false,true,false]`, true, false},
		{"boolSliceIndex", `{ .bools[0] }`, false, types, `true`, true, false},
		{"boolSliceIndex", `{ .bools[-1] }`, false, types, `false`, true, false},
		{"boolSubSlice", `{ .bools[0:2] }`, false, types, `true false`, true, false},
		{"boolSubSliceFirst2", `{ .bools[:2] }`, false, types, `true false`, true, false},
		{"boolSubSliceStep2", `{ .bools[:4:2] }`, false, types, `true true`, true, false},
		// integer slice tests
		{"integerSlice", `{ .integers }`, false, types, `[1,2,3,4]`, true, false},
		{"integerSliceIndex", `{ .integers[0] }`, false, types, `1`, true, false},
		{"integerSliceIndexReverse", `{ .integers[-2] }`, false, types, `3`, true, false},
		{"integerSubSliceFirst2", `{ .integers[0:2] }`, false, types, `1 2`, true, false},
		{"integerSubSliceFirst2Alt", `{ .integers[:2] }`, false, types, `1 2`, true, false},
		{"integerSubSliceStep2", `{ .integers[:4:2] }`, false, types, `1 3`, true, false},
		// float slice tests
		// todo float format can be configure on JSONPath - the test is configured to use "%.2f"
		{"floatSlice", `{ .floats }`, false, types, `[1.00,2.20,3.30,4.00]`, true, false},
		{"floatSliceIndex", `{ .floats[0] }`, false, types, `1.00`, true, false},
		{"floatSliceIndexReverse", `{ .floats[-2] }`, false, types, `3.30`, true, false},
		{"floatSubSliceFirst2", `{ .floats[0:2] }`, false, types, `1.00 2.20`, true, false},
		{"floatSubSliceFirst2Alt", `{ .floats[:2] }`, false, types, `1.00 2.20`, true, false},
		{"floatSubSliceStep2", `{ .floats[:4:2] }`, false, types, `1.00 3.30`, true, false},
		// strings slice tests
		{"stringSlice", `{ .strings }`, false, types, `["one","two","three","four"]`, true, false},
		{"stringSliceIndex", `{ .strings[0] }`, false, types, `one`, true, false},
		{"stringSliceIndexReverse", `{ .strings[-2] }`, false, types, `three`, true, false},
		{"stringSubSliceFirst2", `{ .strings[0:2] }`, false, types, `one two`, true, false},
		{"stringSubSliceFirst2Alt", `{ .strings[:2] }`, false, types, `one two`, true, false},
		{"stringSubSliceStep2", `{ .strings[:4:2] }`, false, types, `one three`, true, false},
		// interfaces slice tests
		// todo float format can be configure on JSONPath - the test is configured to use "%.2f"
		{"interfaceSlice", `{ .interfaces }`, false, types, `[true,"one",1,1.10]`, true, false},
		{"interfaceSliceIndex", `{ .interfaces[0] }`, false, types, `true`, true, false},
		{"interfaceSliceIndexReverse", `{ .interfaces[-2] }`, false, types, `1`, true, false},
		{"interfaceSubSliceFirst2", `{ .interfaces[0:2] }`, false, types, `true one`, true, false},
		{"interfaceSubSliceFirst2Alt", `{ .interfaces[:2] }`, false, types, `true one`, true, false},
		{"interfaceSubSliceStep2", `{ .interfaces[:4:2] }`, false, types, `true 1`, true, false},
		// maps slice tests
		{"mapSlice", `{ .maps }`, false, types,
			`[{"name":"one","value":1},{"name":"two","value":2.02},{"name":"three","value":3.03},{"name":"four","value":4.04}]`, false, false},
		{"mapSliceIndex", `{ .maps[0] }`, false, types, `{"name":"one","value":1}`, false, false},
		{"mapSliceIndexReverse", `{ .maps[-2] }`, false, types, `{"name":"three","value":3.03}`, false, false},
		{"mapSubSliceFirst2", `{ .maps[0:2] }`, false, types, `{"name":"one","value":1} {"name":"two","value":2.02}`, false, false},
		{"mapSubSliceFirst2Alt", `{ .maps[:2] }`, false, types, `{"name":"one","value":1} {"name":"two","value":2.02}`, false, false},
		{"mapSubSliceStepOdd", `{ .maps[::2] }`, false, types, `{"name":"one","value":1} {"name":"three","value":3.03}`, false, false},
		{"mapSubSliceStepEven", `{ .maps[1::2] }`, false, types, `{"name":"two","value":2.02} {"name":"four","value":4.04}`, false, false},
		// structs slice tests
		// todo float format can be configure on JSONPath - the test is configured to use "%.2f"
		{"structSlice", `{ .structs }`, false, types,
			`[{"name":"one","value":1,"type":"integer"},{"name":"two","value":2.00,"type":"float"},{"name":"three","value":3,"type":"integer"},{"name":"four","value":4.00,"type":"float"}]`, true, false},
		{"structSliceIndex", `{ .structs[0] }`, false, types, `{"name":"one","value":1,"type":"integer"}`, true, false},
		{"structSliceIndexReverse", `{ .structs[-2] }`, false, types, `{"name":"three","value":3,"type":"integer"}`, true, false},
		{"structSubSliceFirst2", `{ .structs[0:2] }`, false, types,
			`{"name":"one","value":1,"type":"integer"} {"name":"two","value":2.00,"type":"float"}`, true, false},
		{"structSubSliceFirst2Alt", `{ .structs[:2] }`, false, types,
			`{"name":"one","value":1,"type":"integer"} {"name":"two","value":2.00,"type":"float"}`, true, false},
		{"structSubSliceStepOdd", `{ .structs[::2] }`, false, types,
			`{"name":"one","value":1,"type":"integer"} {"name":"three","value":3,"type":"integer"}`, true, false},
		{"structSubSliceStepEven", `{ .structs[1::2] }`, false, types,
			`{"name":"two","value":2.00,"type":"float"} {"name":"four","value":4.00,"type":"float"}`, true, false},
	}

	testJSONPath(sliceTests, t)
}

type book struct {
	Category string
	Author   string
	Title    string
	Price    float32
}

func (b book) string() string {
	return fmt.Sprintf("{Category: %s, Author: %s, Title: %s, Price: %v}", b.Category, b.Author, b.Title, b.Price)
}

type bicycle struct {
	Color string
	Price float32
	IsNew bool
}

type empName string
type job string
type store struct {
	Book      []book
	Bicycle   []bicycle
	Name      string
	Labels    map[string]int
	Employees map[empName]job
}

func TestStructInput(t *testing.T) {

	storeData := store{
		Name: "jsonpath",
		Book: []book{
			{"reference", "Nigel Rees", "Sayings of the Centurey", 8.95},
			{"fiction", "Evelyn Waugh", "Sword of Honour", 12.99},
			{"fiction", "Herman Melville", "Moby Dick", 8.99},
		},
		Bicycle: []bicycle{
			{"red", 19.95, true},
			{"green", 20.01, false},
		},
		Labels: map[string]int{
			"engieer":  10,
			"web/html": 15,
			"k8s-app":  20,
		},
		Employees: map[empName]job{
			"jason": "manager",
			"dan":   "clerk",
		},
	}

	storeTests := []jsonpathTest{
		{"plain", "hello jsonpath", false, nil, "hello jsonpath", true, false},

		// Todo INVALID
		//{"recursive", "{..}", []int{1, 2, 3}, "[1,2,3]", true, false}, ... possible alternatives
		{"recursive", "{..*}", false, []int{1, 2, 3}, "1 2 3", true, false},
		{"wildcard I", "{.*}", false, []int{1, 2, 3}, "1 2 3", true, false},

		// todo INVALID ... without curly braces, this is just parsed as normal text!
		//{"same as input", "$", []int{1, 2, 3}, "[1,2,3]", true, false},
		{"same as input", "{$}", false, []int{1, 2, 3}, "[1,2,3]", true, false},

		{"filter", "{[?(@<5)]}", false, []int{2, 6, 3, 7}, "2 3", true, false},
		{"quote", `{"{"}`, false, nil, "{", true, false},
		{"union", "{[1,3,4]}", false, []int{0, 1, 2, 3, 4}, "1 3 4", true, false},
		{"array", "{[0:2]}", false, []string{"Monday", "Tudesday"}, "Monday Tudesday", true, false},
		{"variable", "hello {.Name}", false, storeData, "hello jsonpath", true, false},

		// TODO invalid - '/' requires quotes
		//{"dict", "{$.Labels.web/html}", storeData, "15", true, false},
		{"dict", "{$.Labels.'web/html'}", false, storeData, "15", true, false},

		{"dict (2)", "{$.Employees.jason}", false, storeData, "manager", true, false},
		{"dict (3)", "{$.Employees.dan}", false, storeData, "clerk", true, false},

		// TODO invalid - '-' requires quotes
		//{"dict (4)", "{.Labels.k8s-app}", storeData, "20", true, false},
		{"dict (4)", "{.Labels.'k8s-app'}", false, storeData, "20", true, false},

		{"nest", "{.Bicycle[*].Color}", false, storeData, "red green", true, false},
		{"allarray", "{.Book[*].Author}", false, storeData, "Nigel Rees Evelyn Waugh Herman Melville", true, false},

		{"allfields", `{range .Bicycle[*]}{ "{" }{ @.* }{ "} " }{end}`, false, storeData, "{red 19.95 true} {green 20.01 false} ", true, false},
		{"recurfields", "{..Price}", false, storeData, fmt.Sprintf("%g %g %g %g %g", 8.95, 12.99, 8.99, 19.95, 20.01), true, false},
		{"recurdotfields", "{..Price}", false, storeData, "8.95 12.99 8.99 19.95 20.01", true, false},

		// todo fails upon parsing already ... not upon execution - this here assumes successful parsing and failure in execution! => move to other testset
		//{"superrecurfields", "{............................................................Price}", storeData, "", true, true},

		{"allstructsSlice", "{.Bicycle}", false, storeData,
			`[{"Color":"red","Price":19.95,"IsNew":true},{"Color":"green","Price":20.01,"IsNew":false}]`, true, false},

		{"allstructs", `{range .Bicycle[*]}{ @ }{ " " }{end}`, false, storeData,
			`{"Color":"red","Price":19.95,"IsNew":true} {"Color":"green","Price":20.01,"IsNew":false} `, true, false},

		{"lastarray", "{.Book[-1:]}", false, storeData,
			`{"Category":"fiction","Author":"Herman Melville","Title":"Moby Dick","Price":8.99}`, true, false},

		{"recurarray", "{..Book[2]}", false, storeData,
			`{"Category":"fiction","Author":"Herman Melville","Title":"Moby Dick","Price":8.99}`, true, false},

		{"bool", "{.Bicycle[?(@.IsNew==true)]}", false, storeData, `{"Color":"red","Price":19.95,"IsNew":true}`, true, false},
	}

	testJSONPath(storeTests, t)

	missingKeyTests := []jsonpathTest{
		{"nonexistent field", "{.hello}", false, storeData, "", true, false},
		{"nonexistent field (2)", "before-{.hello}after", false, storeData, "before-after", true, false},
	}
	testFailJSONPath(missingKeyTests, t)

	failStoreTests := []jsonpathTest{
		// fails during parsing already => moved to fail-parsing-test
		//{"invalid identifier", "{hello}", storeData, "unrecognized identifier hello", true, false},

		// todo allowMissingField => false
		{"nonexistent field (3)", "{.hello}", false, storeData, "hello is not found", true, false},

		{"invalid array", "{.Labels[0]}", false, storeData, "map[string]int is not array or slice", true, false},

		// fails during parsing already => moved to fail-parsing-test
		//{"invalid filter operator", "{.Book[?(@.Price<>10)]}", storeData, "unrecognized filter operator <>", true, false},

		// fails during parsing already => moved to fail-parsing-test
		//{"redundant end", "{range .Labels.*}{@}{end}{end}", storeData, "not in range, nothing to end", true, false},
	}
	testFailJSONPath(failStoreTests, t)
}

func TestJSONInput(t *testing.T) {
	var pointsJSON = []byte(`[
		{"id": "i1", "x":  4, "y": -5},
		{"id": "i2", "x": -2, "y": -5, "z":1},
		{"id": "i3", "x":  8, "y":  3 },
		{"id": "i4", "x": -6, "y": -1 },
		{"id": "i5", "x":  0, "y":  2, "z": 1 },
		{"id": "i6", "x":  1, "y":  4 }
	]`)
	var pointsData interface{}
	err := json.Unmarshal(pointsJSON, &pointsData)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)
	pointsTests := []jsonpathTest{
		{"exists filter", "{[?(@.z)].id}", true, pointsData, "i2 i5", true, false},
		{"bracket key", "{[0]['id']}", false, pointsData, "i1", true, false},
	}
	testJSONPath(pointsTests, t)
}

// TestKubernetes tests some use cases from kubernetes
func TestKubernetes(t *testing.T) {
	var input = []byte(`{
	  "kind": "List",
	  "items":[
		{
		  "kind":"None",
		  "metadata":{
		    "name":"127.0.0.1",
			"labels":{
			  "kubernetes.io/hostname":"127.0.0.1"
			}
		  },
		  "status":{
			"capacity":{"cpu":"4"},
			"ready": true,
			"addresses":[{"type": "LegacyHostIP", "address":"127.0.0.1"}]
		  }
		},
		{
		  "kind":"None",
		  "metadata":{
			"name":"127.0.0.2",
			"labels":{
			  "kubernetes.io/hostname":"127.0.0.2"
			}
		  },
		  "status":{
			"capacity":{"cpu":"8"},
			"ready": false,
			"addresses":[
			  {"type": "LegacyHostIP", "address":"127.0.0.2"},
			  {"type": "another", "address":"127.0.0.3"}
			]
		  }
		}
	  ],
	  "users":[
	    {
	      "name": "myself",
	      "user": {}
	    },
	    {
	      "name": "e2e",
	      "user": {"username": "admin", "password": "secret"}
	  	}
	  ]
	}`)
	var nodesData interface{}
	err := json.Unmarshal(input, &nodesData)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)

	nodesTests := []jsonpathTest{
		{"range item", `{range .items[*]}{.metadata.name}, {end}{.kind}`, false, nodesData, "127.0.0.1, 127.0.0.2, List", true, false},
		{"range item with quote", "{range .items[*]}{.metadata.name}{\"\t\"}{end}", false, nodesData, "127.0.0.1\t127.0.0.2\t", true, false},
		{"range addresss", `{.items[*].status.addresses[*].address}`, false, nodesData,
			"127.0.0.1 127.0.0.2 127.0.0.3", true, false},
		{"double range", `{range .items[*]}{range .status.addresses[*]}{.address}, {end}{end}`, false, nodesData,
			"127.0.0.1, 127.0.0.2, 127.0.0.3, ", true, false},
		{"item name", `{.items[*].metadata.name}`, false, nodesData, "127.0.0.1 127.0.0.2", true, false},

		// TODO invalid interpreted '.' WITHIN quoted string
		//{"union nodes capacity", `{.items[*]['metadata.name', 'status.capacity']}`, false, nodesData,
		//	`127.0.0.1 127.0.0.2 {"cpu":"4"} {"cpu":"8"}`, true, false},

		{"range nodes capacity", `{range .items[*]}[{.metadata.name}, {.status.capacity}] {end}`, false, nodesData,
			`[127.0.0.1, {"cpu":"4"}] [127.0.0.2, {"cpu":"8"}] `, true, false},
		{"user password", `{.users[?(@.name=="e2e")].user.password}`, false, &nodesData, "secret", true, false},

		// todo invalid: special chars ('.' and '/') in unquoted name selector
		//{"hostname", `{.items[0].metadata.labels.kubernetes\.io/hostname}`, false, &nodesData, "127.0.0.1", true, false},
		{"hostname", `{.items[0].metadata.labels.'kubernetes.io/hostname'}`, false, &nodesData, "127.0.0.1", true, false},

		// todo invalid: special chars ('.' and '/') in unquoted name selector
		//{"hostname filter", `{.items[?(@.metadata.labels.kubernetes\.io/hostname=="127.0.0.1")].kind}`, false, &nodesData, "None", true, false},
		{"hostname filter", `{.items[?(@.metadata.labels.'kubernetes.io/hostname'=="127.0.0.1")].kind}`, false, &nodesData, "None", true, false},

		{"bool item less", `{.items[?(@..ready)].metadata.name}`, true, &nodesData, "127.0.0.1 127.0.0.2", true, false},
		{"bool item", `{.items[?(@..ready==true)].metadata.name}`, false, &nodesData, "127.0.0.1", true, false},
	}
	testJSONPath(nodesTests, t)

	randomPrintOrderTests := []jsonpathTest{
		{"recursive name", "{..name}", false, nodesData, `127.0.0.1 127.0.0.2 myself e2e`, true, false},
	}
	testJSONPathSortOutput(randomPrintOrderTests, t)
}

func TestEmptyRange(t *testing.T) {
	var input = []byte(`{"items":[]}`)
	var emptyList interface{}
	err := json.Unmarshal(input, &emptyList)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)

	tests := []jsonpathTest{
		{"empty range", `{range .items[*]}{.metadata.name}{end}`, false, &emptyList, "", true, false},
		{"empty nested range", `{range .items[*]}{.metadata.name}{":"}{range @.spec.containers[*]}{.name}{","}{end}{"+"}{end}`, false, &emptyList, "", true, false},
	}
	testJSONPath(tests, t)
}

func TestNestedRanges(t *testing.T) {
	var input = []byte(`{
		"items": [
			{
				"metadata": {
					"name": "pod1"
				},
				"spec": {
					"containers": [
						{
							"name": "foo",
							"another": [
								{ "name": "value1" },
								{ "name": "value2" }
							]
						},
						{
							"name": "bar",
							"another": [
								{ "name": "value1" },
								{ "name": "value2" }
							]
						}
					]
            }
			},
			{
				"metadata": {
					"name": "pod2"
				},
				"spec": {
					"containers": [
						{
							"name": "baz",
							"another": [
								{ "name": "value1" },
								{ "name": "value2" }
							]
						}
					]
            }
			}
		]
	}`)
	var data interface{}
	err := json.Unmarshal(input, &data)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)

	testJSONPath([]jsonpathTest{
		{
			"nested range with a trailing newline",
			`{range .items[*]}` +
				`{.metadata.name}` +
				`{":"}` +
				`{range @.spec.containers[*]}` +
				`{.name}` +
				`{","}` +
				`{end}` +
				`{"+"}` +
				`{end}`,
			false, data,
			"pod1:foo,bar,+pod2:baz,+",
			true,
			false,
		},
	}, t)

	testJSONPath([]jsonpathTest{
		{
			"nested range with a trailing character within another nested range with a trailing newline",
			`{range .items[*]}` +
				`{.metadata.name}` +
				`{"~"}` +
				`{range @.spec.containers[*]}` +
				`{.name}` +
				`{":"}` +
				`{range @.another[*]}` +
				`{.name}` +
				`{","}` +
				`{end}` +
				`{"+"}` +
				`{end}` +
				`{"#"}` +
				`{end}`,
			false, data,
			"pod1~foo:value1,value2,+bar:value1,value2,+#pod2~baz:value1,value2,+#",
			true,
			false,
		},
	}, t)

	testJSONPath([]jsonpathTest{
		{
			"two nested ranges at the same level with a trailing newline",
			`{range .items[*]}{.metadata.name}` + "{\"\t\"}" + `{range @.spec.containers[*]}{.name}{" "}{end}` + "{\"\t\"}" + `{range @.spec.containers[*]}{.name}{" "}{end}` + "{\"\n\"}" + `{end}`,
			false, data,
			"pod1\tfoo bar \tfoo bar \npod2\tbaz \tbaz \n",
			true,
			false,
		},
	}, t)
}

func TestFilterPartialMatchesSometimesMissingAnnotations(t *testing.T) {
	// for https://issues.k8s.io/45546
	var input = []byte(`{
		"kind": "List",
		"items": [
			{
				"kind": "Pod",
				"metadata": {
					"name": "pod1",
					"annotations": {
						"color": "blue"
					}
				}
			},
			{
				"kind": "Pod",
				"metadata": {
					"name": "pod2"
				}
			},
			{
				"kind": "Pod",
				"metadata": {
					"name": "pod3",
					"annotations": {
						"color": "green"
					}
				}
			},
			{
				"kind": "Pod",
				"metadata": {
					"name": "pod4",
					"annotations": {
						"color": "blue"
					}
				}
			}
		]
	}`)
	var data interface{}
	err := json.Unmarshal(input, &data)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)

	testJSONPath([]jsonpathTest{
		{
			"filter, should only match a subset, some items don't have annotations, tolerate missing items",
			`{.items[?(@.metadata.annotations.color=="blue")].metadata.name}`,
			true, data,
			"pod1 pod4",
			true,
			false, // expect no error
		},
	}, t)

	testFailJSONPath([]jsonpathTest{
		{
			"filter, should only match a subset, some items don't have annotations, error on missing items",
			`{.items[?(@.metadata.annotations.color=="blue")].metadata.name}`,
			false, data,
			"",
			true,
			true, // expect an error
		},
	}, t)
}

func TestNegativeIndex(t *testing.T) {
	var input = []byte(
		`{
			"apiVersion": "v1",
			"kind": "Pod",
			"spec": {
				"containers": [
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake0"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake1"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake2"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake3"
					}]}}`)

	var data interface{}
	err := json.Unmarshal(input, &data)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)

	testJSONPath([]jsonpathTest{
		{
			"test containers[0], it equals containers[0]",
			`{.spec.containers[0].name}`,
			false, data,
			"fake0",
			true,
			false,
		},
		{
			"test containers[0:0], it equals the empty set",
			`{.spec.containers[0:0].name}`,
			false, data,
			"",
			true,
			false,
		},
		{
			"test containers[0:-1], it equals containers[0:3]",
			`{.spec.containers[0:-1].name}`,
			false, data,
			"fake0 fake1 fake2",
			true,
			false,
		},

		// TODO will not fail, but return an empty set (with a negative step, the query would return results in reverse order)
		{
			"test containers[-1:0], expect error",
			`{.spec.containers[-1:0].name}`,
			false, data,
			"",
			true,
			false,
		},

		{
			"test containers[-1], it equals containers[3]",
			`{.spec.containers[-1].name}`,
			false, data,
			"fake3",
			true,
			false,
		},
		{
			"test containers[-1:], it equals containers[3:]",
			`{.spec.containers[-1:].name}`,
			false, data,
			"fake3",
			true,
			false,
		},
		{
			"test containers[-2], it equals containers[2]",
			`{.spec.containers[-2].name}`,
			false, data,
			"fake2",
			true,
			false,
		},
		{
			"test containers[-2:], it equals containers[2:]",
			`{.spec.containers[-2:].name}`,
			false, data,
			"fake2 fake3",
			true,
			false,
		},
		{
			"test containers[-3], it equals containers[1]",
			`{.spec.containers[-3].name}`,
			false, data,
			"fake1",
			true,
			false,
		},
		{
			"test containers[-4], it equals containers[0]",
			`{.spec.containers[-4].name}`,
			false, data,
			"fake0",
			true,
			false,
		},
		{
			"test containers[-4:], it equals containers[0:]",
			`{.spec.containers[-4:].name}`,
			false, data,
			"fake0 fake1 fake2 fake3",
			true,
			false,
		},
		{
			"test containers[5:5], expect empty set",
			`{.spec.containers[5:5].name}`,
			false, data,
			"",
			true,
			false,
		},

		// TODO will not fail, but return an empty set (with a negative step, the query would return results in reverse order)
		{
			"test containers[-5:-5], expect empty set",
			`{.spec.containers[-5:-5].name}`,
			false, data,
			"",
			false,
			false,
		},

		// TODO will not fail, but return an empty set (with a negative step, the query would return results in reverse order)
		{
			"test containers[3:1], expect a error cause start index is greater than end index",
			`{.spec.containers[3:1].name}`,
			false, data,
			"",
			true,
			false,
		},

		// TODO will not fail, but return an empty set (with a negative step, the query would return results in reverse order)
		{
			"test containers[-1:-2], it equals containers[3:2], expect a error cause start index is greater than end index",
			`{.spec.containers[-1:-2].name}`,
			false, data,
			"",
			true,
			false,
		},
	}, t)

	testFailJSONPath([]jsonpathTest{
		// actually failing for misssingKey (index out of bounds)
		{
			"test containers[-5], expect a error cause it out of bounds",
			`{.spec.containers[-5].name}`,
			false, data,
			"",
			true,
			true, // expect error
		},
	}, t)
}

func TestRunningPodsJSONPathOutput(t *testing.T) {
	var input = []byte(`{
		"kind": "List",
		"items": [
			{
				"kind": "Pod",
				"metadata": {
					"name": "pod1"
				},
				"status": {
						"phase": "Running"
				}
			},
			{
				"kind": "Pod",
				"metadata": {
					"name": "pod2"
				},
				"status": {
						"phase": "Running"
				}
			},
			{
				"kind": "Pod",
				"metadata": {
					"name": "pod3"
				},
				"status": {
						"phase": "Running"
				}
			},
       		{
				"resourceVersion": ""
			}
		]
	}`)
	var data interface{}
	err := json.Unmarshal(input, &data)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)

	testJSONPath([]jsonpathTest{
		{
			"range over pods without selecting the last one",
			`{range .items[?(.status.phase=="Running")]}{.metadata.name}{" is Running` + "\n" + `"}{end}`,
			true, data,
			"pod1 is Running\npod2 is Running\npod3 is Running\n",
			true,
			false, // expect no error
		},
	}, t)
}

func TestStep(t *testing.T) {
	var input = []byte(
		`{
			"apiVersion": "v1",
			"kind": "Pod",
			"spec": {
				"containers": [
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake0"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake1"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake2"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake3"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake4"
					},
					{
						"image": "radial/busyboxplus:curl",
						"name": "fake5"
					}]}}`)

	var data interface{}
	err := json.Unmarshal(input, &data)
	require.Nilf(t, err, "could not unmarshal json test input: %v", err)

	testJSONPath([]jsonpathTest{
		{
			"test containers[0:], it equals containers[0:6:1]",
			`{.spec.containers[0:].name}`,
			false, data,
			"fake0 fake1 fake2 fake3 fake4 fake5",
			true,
			false,
		},
		{
			"test containers[0:6:], it equals containers[0:6:1]",
			`{.spec.containers[0:6:].name}`,
			false, data,
			"fake0 fake1 fake2 fake3 fake4 fake5",
			true,
			false,
		},
		{
			"test containers[0:6:1]",
			`{.spec.containers[0:6:1].name}`,
			false, data,
			"fake0 fake1 fake2 fake3 fake4 fake5",
			true,
			false,
		},

		// todo will not fail, spec defines empty result for step==0
		{
			"test containers[0:6:0], it errors",
			`{.spec.containers[0:6:0].name}`,
			false, data,
			"",
			true,
			false,
		},

		// todo will not fail, BUT return an empty set (empty slice in reverse order)
		{
			"test containers[0:6:-1], it errors",
			`{.spec.containers[0:6:-1].name}`,
			false, data,
			"",
			true,
			false,
		},
		{
			"test containers[1:4:2]",
			`{.spec.containers[1:4:2].name}`,
			false, data,
			"fake1 fake3",
			true,
			false,
		},
		{
			"test containers[1:4:3]",
			`{.spec.containers[1:4:3].name}`,
			false, data,
			"fake1",
			true,
			false,
		},
		{
			"test containers[1:4:4]",
			`{.spec.containers[1:4:4].name}`,
			false, data,
			"fake1",
			true,
			false,
		},
		{
			"test containers[0:6:2]",
			`{.spec.containers[0:6:2].name}`,
			false, data,
			"fake0 fake2 fake4",
			true,
			false,
		},
		{
			"test containers[0:6:3]",
			`{.spec.containers[0:6:3].name}`,
			false, data,
			"fake0 fake3",
			true,
			false,
		},
		{
			"test containers[0:6:5]",
			`{.spec.containers[0:6:5].name}`,
			false, data,
			"fake0 fake5",
			true,
			false,
		},
		{
			"test containers[0:6:6]",
			`{.spec.containers[0:6:6].name}`,
			false, data,
			"fake0",
			true,
			false,
		},
	}, t)
}

func TestNastyChars(t *testing.T) {
	var input = []byte(
		`{
			"foo": "bar",
			"has space": "expected value",
			"nested": {
				"nested name": "nested value"
			},
			"has,comma": "expected comma value",
			"unicode\u004b": "expect K==\u004b",
			"UNICODE\u004B": "expect K=\u004B"
		}`)

	var data interface{}
	err := json.Unmarshal(input, &data)
	require.Nilf(t, err, "could not unmarshal json test input: %#v", err)

	testJSONPath([]jsonpathTest{
		{
			"test top level property name containing space starting at root",
			`{$['has space']}`,
			false, data,
			"expected value",
			true,
			false,
		},
		{
			"test top level property name containing space starting at current",
			`{@['has space']}`,
			false, data,
			"expected value",
			true,
			false,
		},
		{
			"test child property name containing space in bracket notation",
			`{$..['nested name']}`,
			false, data,
			"nested value",
			true,
			false,
		},
		{
			"test child property name containing space dot notation",
			`{$.nested.'nested name'}`,
			false, data,
			"nested value",
			true,
			false,
		},
		{
			"test property name containing comma",
			`{.'has,comma'}`,
			false, data,
			"expected comma value",
			true,
			false,
		},
		{
			"test double quoted name literal",
			`{."foo"}`,
			false, data,
			"bar",
			true,
			false,
		},
		{
			"test unicode lower case hex",
			`{."unicode\u004b"}`,
			false, data,
			"expect K==\u004b",
			true,
			false,
		},
		{
			"test unicode upper case hex",
			`{."unicode\u004B"}`,
			false, data,
			"expect K==\u004B",
			true,
			false,
		},
	}, t)
}
