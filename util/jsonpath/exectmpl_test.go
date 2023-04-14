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

// from https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-12.html
// partially using maps => do not care about order of resultSet !!!

type execTmplTest struct {
	tmpl                   string
	name                   string
	data                   interface{}
	format                 resultFormat
	expectedResultOrLength interface{}

	// if order of result not guaranteed (e.g. for map usage, who do not guarantee any iteration order): only assert equal charHistograms
	orderedResultExpected bool

	allowMissingKey bool
}

func exampleTestData() interface{} {
	return map[string]interface{}{
		"kind": "List",
		"items": []map[string]interface{}{
			{
				"kind":     "None",
				"metadata": map[string]string{"name": "127.0.0.1"},
				"status": map[string]interface{}{
					"capacity": map[string]interface{}{"cpu": "4"},
					"addresses": []map[string]interface{}{
						{"type": "LegacyHostIP", "address": "127.0.0.1"},
					},
				},
			},
			{
				"kind":     "None",
				"metadata": map[string]string{"name": "127.0.0.2"},
				"status": map[string]interface{}{
					"capacity": map[string]interface{}{"cpu": "8"},
					"addresses": []map[string]interface{}{
						{"type": "LegacyHostIP", "address": "127.0.0.2"},
						{"type": "another", "address": "127.0.0.3"},
					},
				},
			},
		},
		"users": []map[string]interface{}{
			{
				"name": "myself",
				"user": map[string]interface{}{},
			},
			{
				"name": "e2e",
				"user": map[string]string{"username": "admin", "password": "secret"},
			},
		},
	}
}

var legacyFormat = resultFormat{legacyFormatted, "%.2f"}

var execTmplTests = []execTmplTest{
	{"kind is {.kind}", "simple text and jsonpath singular-query", exampleTestData(), legacyFormat, "kind is List", true, false},
	{"{@}", "the same as input", exampleTestData(), resultFormat{jsonFormatted, "%.2f"}, 1022 /*all json formmatted - will fail on changing formatting :-( */, false, false},
	{"{.kind}", "dot segment notation", exampleTestData(), legacyFormat, "List", true, false},
	{"{[kind]}", "square bracket segment notation", exampleTestData(), legacyFormat, "List", true, false},

	{"{..name}", "recursive/descending", exampleTestData(), legacyFormat, "127.0.0.1 127.0.0.2 myself e2e", false, false},

	{"{.items[*].metadata.name}", "wildcard-selector", exampleTestData(), legacyFormat, "127.0.0.1 127.0.0.2", true, false},

	{"{.users[0].name}", "index-selector", exampleTestData(), legacyFormat, "myself", true, false},

	// WILL NEVER WORK this way. as '.' WITHIN quotes is interpreted as normal char and part of the string. this CONTRADICTS any JSONPath spec ... alternative: use range-end template-operator
	// NO GO {"{.items[*]['metadata.name', 'status.capacity']}", "union???", exampleTestData(), legacyFormatted, "[127.0.0.1, map[cpu:4]] [127.0.0.2, map[cpu:8]]", false},
	// TODO example in refPage differs from existing tests in jsonpath_test.go ?!?
	{"{range .items[*]}[{.metadata.name}, {.status.capacity}] {end}", "range-end-operator with mixed text and jsonpath elements", exampleTestData(), legacyFormat, "[127.0.0.1, {\"cpu\":\"4\"}] [127.0.0.2, {\"cpu\":\"8\"}] ", true, false},

	{"{.users[?(@.name==\"e2e\")].user.password}", "filter", exampleTestData(), legacyFormat, "secret", true, false},
	{"{range .items[*]}{.metadata.name}{'\t'}{end}", "quoted string", exampleTestData(), legacyFormat, "127.0.0.1\t127.0.0.2\t", true, false},

	{"{range .items[*]}relative path kind {.kind}; root kind {$.kind}{\"\n\"}{end}", "relative and absolute path template queries", exampleTestData(), legacyFormat, "relative path kind None; root kind List\nrelative path kind None; root kind List\n", true, false},
}

func TestExecTmpl(t *testing.T) {
	const printDebugMsgs = false
	for _, test := range execTmplTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newTemplateParser(test.name)
			err := parser.parse(test.tmpl)
			require.Nilf(subT, err, "failed to initialize test - while parsing the template - with unexpected error: %v", err)
			fmt.Printf("\ntemplate : %s", parser.string())
			fmt.Printf("\nexpected : %v", test.expectedResultOrLength)

			var b bytes.Buffer
			rootDataNode := reflect.ValueOf(test.data)
			err = executeTemplate(&b, test.format, parser, rootDataNode, test.allowMissingKey, newFunctionRegistry(), printDebugMsgs)
			require.Nilf(subT, err, "failed with unexpected error: %v", err)

			result := b.String()

			fmt.Printf("\nresult  : >>%s<<\n\n", result)
			requireExpectedString(subT, test.expectedResultOrLength, result, test.orderedResultExpected)
			fmt.Print("\n\n")
			println("\n")
		})
	}
}
