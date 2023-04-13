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

// package jsonpath is a template engine using jsonpath syntax,
// which can be seen at https://datatracker.ietf.org/doc/draft-ietf-jsonpath-base/ (V12; last updated 2023-03-26).
// In addition, it has {range} {end} fct to iterate list and slice.

package jsonpath // import "k8s.io/client-go/util/jsonpath"

// todo
//implementation follows
//* specs from https://datatracker.ietf.org/doc/draft-ietf-jsonpath-base/ V12 (last updated 2023-03-26)
//=> '$' and '@' are mandatory - we keep them optional (see other spec below)
//=> allow double or single quotes to be used for name-selectors and string-literals
//=> according to https://www.ietf.org/archive/id/draft-ietf-jsonpath-base-12.html#extest
//filter-queries by their own are existence-queries and are optimized to break up execution after finding any
//result - testing an explicit value would require the use of a comparison operator
//* specs from https://kubernetes.io/docs/reference/kubectl/jsonpath/
//=> additionally defines range-end operator to iterate over result lists
//=> here '$' - and '@'? - is declared optional
//=> requires double quote to be used for text inside a JSONPath query - see support single and double (see spec above)

//design goals of refactoring
//* implementation on JSON source data being in memory in parsed state as nested structs/arrays/maps containing values
//* implementation will traverse the JSON structure potentially many times
//  - make use of existence query to optimize and abort traversal early

// TODO
// * describe usage
// * some of the std fcts ...
// * error handling
//   - have common errors for parsing (SyntaxError), execution (ExecutionError)
//   - review error msgs
// * ? add warnings for suspicious queries
//   - empty slices on execution
//   - non-singular-queries used for comparison => actually spec would require that!
// * all todos
// * optimizations:
//   . index & name selectors without descending can abort cur Node after hit => "there can only be one"
// * ? generics for Singular

// TODO release notes
// * needs update of examples on reference page in wiki!
// * some cases that are not valid any more ... see todos in tests!
//   - '..' NOT valid any more!
//   - JSONPath name-selectors must be quoted individually! ['metadata.name'] looks for a field/node with name/key 'metadata.name'
//     and NOT first for a field/node with name/key 'metadata' and subsequently for a result's field/node with name/key 'data'!
//   - problem examples:
//     . `{['book.price']}`                                => `{.book.price}`
//     . `{['bicycle.price', 3, 'book.price']}`            => `{.bicycle.price}{[3]}{.book.price}`
//     . `{.items[*]['metadata.name', 'status.capacity']}` => `{range .items.*}{@.metadata.name}{.status.capacity}{end}`
//
// * topic: output order might depend on input data type: structs have and order for their fields, maps DON'T
//          therefore the usage of maps will result in random output order!
