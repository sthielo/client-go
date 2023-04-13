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
	"reflect"
	"testing"
)

type parseQueryTest struct {
	name string
	text string
	root *nodeIdentifier
}

var dummyInnerParser = &innerParser{"", 0, 0, 0, 0}

var parseQryTests = []parseQueryTest{
	{"rootNodeIdentifierOnly", `$`, &nodeIdentifier{rootNodeSymbol, []segment{}}},
	{"currNodeIdentifierOnly", `@`, &nodeIdentifier{currentNodeSymbol, []segment{}}},
	{"dotNameSelector", `$.abc`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"currNodeDotNameSelector", `@.'abc'`, &nodeIdentifier{currentNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"dotNameSelectorMissingRootNodeIdentifier", `.'abc'`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"squareNameSelectorMissingNodeIdentifier", `['abc']`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"dotDescendantNameSelector", `..'abc'`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"currNodeDescendantDotNameSelector", `@..'abc'`, &nodeIdentifier{currentNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"dotDescendantNameSelectorMissingRootNodeIdentifier", `..'abc'`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"descendantSquareNameSelectorMissingNodeIdentifier", `..['abc']`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"descendantSquareNameSelectorMissingNodeIdentifier", `..[abc]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},

	{"dotNameSelectorDoubleQuoted", `."abc"`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"dotNameSelectorSingleQuoted", `.'abc'`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"abc"},
		}},
	}}},
	{"dotWldcardSelector", `.*`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&wildcardSelector{},
		}},
	}}},
	{"squareWildcardSelector", `[*]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&wildcardSelector{},
		}},
	}}},
	{"nameSelectors", `["def", 'ghi', abc]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"def"},
			&nameSelector{"ghi"},
			&nameSelector{"abc"},
		}},
	}}},
	{"indexSelectors", `[3, -7]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&indexSelector{3},
			&indexSelector{-7},
		}},
	}}},
	{"arraySliceSelectors", `[5:7, :5:2, 6::-2]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&arraySliceSelector{optionalInt{true, 5}, optionalInt{true, 7}, 1},
			&arraySliceSelector{optionalInt{false, 0}, optionalInt{true, 5}, 2},
			&arraySliceSelector{optionalInt{true, 6}, optionalInt{false, 0}, -2},
		}},
	}}},
	{"arraySliceSelectorsWithTrailingSpace", `[-2:: ]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&arraySliceSelector{optionalInt{true, -2}, optionalInt{false, 0}, 1},
		}},
	}}},
	{"filterSelectorsDescendantFilterQry", `[?(@.'abc'[*]), ?$[3]..'special', ?.'less'<3]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&filterQry{true, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
				&segmentImpl{childSegmentType, []selector{
					&nameSelector{"abc"},
				}},
				&segmentImpl{childSegmentType, []selector{
					&wildcardSelector{},
				}},
			}}, false, dummyInnerParser}}},
			&filterSelector{&filterQry{true, &queryParser{"filterQry-1", &nodeIdentifier{rootNodeSymbol, []segment{
				&segmentImpl{childSegmentType, []selector{
					&indexSelector{3},
				}},
				&segmentImpl{descendantSegmentType, []selector{
					&nameSelector{"special"},
				}},
			}}, false, dummyInnerParser}}},
			&filterSelector{&compareExpr{
				&filterQry{false, &queryParser{"filterQry-2", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"less"},
					}},
				}}, true, dummyInnerParser}},
				&intLiteral{3},
				ltOp,
			}},
		}},
	}}},
	{"filterCompareSelectorsMultipleSegments", `[?.'greater'>3, ?3==$.'equal'][?.'ge'>="literal", ?['le']<=-5.47e-3]..[?@.'ne'!=true]`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&compareExpr{
				&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"greater"},
					}},
				}}, true, dummyInnerParser}},
				&intLiteral{3},
				gtOp,
			}},
			&filterSelector{&compareExpr{
				&intLiteral{3},
				&filterQry{false, &queryParser{"filterQry-1", &nodeIdentifier{rootNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"equal"},
					}},
				}}, true, dummyInnerParser}},
				eqOp,
			}},
		}},
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&compareExpr{
				&filterQry{false, &queryParser{"filterQry-2", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"ge"},
					}},
				}}, true, dummyInnerParser}},
				&stringLiteral{"literal"},
				geOp,
			}},
			&filterSelector{&compareExpr{
				&filterQry{false, &queryParser{"filterQry-3", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"le"},
					}},
				}}, true, dummyInnerParser}},
				&floatLiteral{-5.47e-3},
				leOp,
			}},
		}},
		&segmentImpl{descendantSegmentType, []selector{
			&filterSelector{&compareExpr{
				&filterQry{false, &queryParser{"filterQry-4", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"ne"},
					}},
				}}, true, dummyInnerParser}},
				&boolLiteral{true},
				neOp,
			}},
		}},
	}}},
	{"nastyChars_Umlaut", "$.'ä'", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"ä"},
		}},
	}}},
	{"nastyChars_Tab", "$.'\t'", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"\t"},
		}},
	}}},
	{"nastyChars_unicode_EUR", "$.'\u20AC'", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"€"},
		}},
	}}},
	{"nastyChars_unicode64_Smiley_\U0001F602", "$.'\U0001F602'", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"\U0001F602"},
		}},
	}}},
	{"nastyChars_backslash", "$.'\\\\'", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"\\"},
		}},
	}}},
	{"nastyChars_SingleQuoteWithinSingleQuotes", "$.'\\''", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"'"},
		}},
	}}},
	{"filterLogicalOpsSelectors", `[?.'greater'&&.'bigger', ?.'small' || ! .'exclude' && .'mini']`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&logicalExpr{
				&filterQry{true, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"greater"},
					}},
				}}, true, dummyInnerParser}},
				&filterQry{true, &queryParser{"filterQry-1", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"bigger"},
					}},
				}}, true, dummyInnerParser}},
				"&&",
			}},
			&filterSelector{&logicalExpr{
				&filterQry{true, &queryParser{"filterQry-2", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"small"},
					}},
				}}, true, dummyInnerParser}},
				&logicalExpr{
					&logicalExpr{
						&filterQry{true, &queryParser{"filterQry-3", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"exclude"},
							}},
						}}, true, dummyInnerParser}},
						nil,
						notOp},
					&filterQry{true, &queryParser{"filterQry-4", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{
							&nameSelector{"mini"},
						}},
					}}, true, dummyInnerParser}},
					andOp,
				},
				orOp,
			}},
		}},
	}}},
	{"function_length", "$[?length(.'name')]]", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&functionExpr{"length", []filterExpr{&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
				&segmentImpl{childSegmentType, []selector{
					&nameSelector{"name"},
				}},
			}}, true, dummyInnerParser}}}}},
		}},
	}}},
	{"function_custom", "$[?custom(.'name')]]", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&functionExpr{"custom", []filterExpr{&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
				&segmentImpl{childSegmentType, []selector{
					&nameSelector{"name"},
				}},
			}}, true, dummyInnerParser}}}}},
		}},
	}}},
	{"logicalComparisonPrio", `..[?.'color'&&.'age' > 5].'age'`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&filterSelector{&logicalExpr{
				&filterQry{true, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"color"},
					}},
				}}, true, dummyInnerParser}},
				&compareExpr{
					&filterQry{false, &queryParser{"filterQry-1", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{
							&nameSelector{"age"},
						}},
					}}, true, dummyInnerParser}},
					&intLiteral{5},
					gtOp,
				},
				andOp,
			}},
		}},
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"age"},
		}},
	}}},
	{"anotherFilterExp", `..[?.'age'>38].'firstName'`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&filterSelector{&compareExpr{
				&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"age"},
					}},
				}}, true, dummyInnerParser}},
				&intLiteral{38},
				gtOp,
			}},
		}},
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"firstName"},
		}},
	}}},
	{"operatorPriorities", "$..[?length(.'firstName')<=3 && custom(.'firstName') < 0.01].'firstName'", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{descendantSegmentType, []selector{
			&filterSelector{&logicalExpr{
				&compareExpr{
					&functionExpr{"length", []filterExpr{&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{
							&nameSelector{"firstName"},
						}},
					}}, true, dummyInnerParser}}}},
					&intLiteral{3},
					leOp,
				},
				&compareExpr{
					&functionExpr{"custom", []filterExpr{&filterQry{false, &queryParser{"filterQry-1", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{
							&nameSelector{"firstName"},
						}},
					}}, true, dummyInnerParser}}}},
					&floatLiteral{0.01},
					ltOp,
				},
				andOp,
			}},
		}},
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"firstName"},
		}},
	}}},
	{"root level filters", "$[?returnarg(.'persons'[*]) <= 'abc']", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&compareExpr{
				&functionExpr{"returnarg", []filterExpr{&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"persons"},
					}},
					&segmentImpl{childSegmentType, []selector{
						&wildcardSelector{},
					}},
				}}, false, dummyInnerParser}}}},
				&stringLiteral{"abc"},
				leOp,
			}},
		}},
	}}},
	{"multiple function arguments", "$[?match(.'firstName', 'Ba.*')]", &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{
				&functionExpr{"match", []filterExpr{
					&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{
							&nameSelector{"firstName"},
						}},
					}}, true, dummyInnerParser}},
					&stringLiteral{"Ba.*"},
				}},
			},
		}},
	}}},
	{"nested query with comparison", `.items[?(@..ready[?@==true])].metadata.name`, &nodeIdentifier{rootNodeSymbol, []segment{
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"items"},
		}},
		&segmentImpl{childSegmentType, []selector{
			&filterSelector{&filterQry{true, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
				&segmentImpl{descendantSegmentType, []selector{
					&nameSelector{"ready"},
				}},
				&segmentImpl{childSegmentType, []selector{
					&filterSelector{&compareExpr{
						&filterQry{false, &queryParser{"filterQry-1", &nodeIdentifier{currentNodeSymbol, []segment{}}, true, dummyInnerParser}},
						&boolLiteral{true},
						eqOp,
					}},
				}},
			}}, false, dummyInnerParser}}},
		}},
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"metadata"},
		}},
		&segmentImpl{childSegmentType, []selector{
			&nameSelector{"name"},
		}},
	}}},
}

func TestParseQuery(t *testing.T) {
	for _, test := range parseQryTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newQueryParser()
			parser.name = test.name
			fmt.Printf("test.name    :%s\n", test.name)
			fmt.Printf("test.query   :%s\n", test.text)
			err := parser.parse(test.text)
			require.Nilf(subT, err, "failed with unexpected error: %v", err)
			fmt.Printf("parse result :%s\n", parser.string())

			require.Conditionf(subT, func() bool {
				return assertEqualNodeIdentifier(subT, test.root, parser.root)
			}, "nodeIdentifier not equal:\ne: %#v\nr: %#v\n", test.root, parser.root)
			println("\n")
		})
	}
}

func assertEqualNodeIdentifier(t *testing.T, e *nodeIdentifier, r *nodeIdentifier) bool {
	require.NotNil(t, r, "nil found instead of a nodeIdentifier instance")
	require.Equal(t, e.nodeIdentifierSymbol, r.nodeIdentifierSymbol, "nodeIdentifierSymbol do not match")
	require.Conditionf(t, func() bool {
		return assertEqualJPSegs(t, e.segments, r.segments)
	}, "jsonpath segments not equal:\ne: %#v\nr: %#v\n", e.segments, r.segments)
	return true
}

func assertEqualJPSegs(t *testing.T, e []segment, r []segment) bool {
	require.Equal(t, len(e), len(r), "len([]segment) do not match")
	for i, eSeg := range e {
		rSeg := r[i]
		require.Equal(t, eSeg.getType(), rSeg.getType(), "types of segment[%d] do not match", i)

		require.Conditionf(t, func() bool {
			return assertEqualSels(t, eSeg.getSelectors(), rSeg.getSelectors())
		}, "[]selector do not match:\ne: %#v\nr: %#v\n", eSeg.getSelectors(), rSeg.getSelectors())
	}
	return true
}

func assertEqualSels(t *testing.T, e []selector, r []selector) bool {
	require.Equal(t, len(e), len(r), "len([]selector) do not match")
	for i, eSel := range e {
		rSel := r[i]
		require.IsTypef(t, reflect.TypeOf(eSel), reflect.TypeOf(rSel), "types of selector[%d] do not match", i)
		require.Conditionf(t, func() bool {
			switch eSel.(type) {
			case *wildcardSelector:
				return true
			case *nameSelector:
				require.Equal(t, eSel.(*nameSelector).name, rSel.(*nameSelector).name)
				return true
			case *indexSelector:
				require.Equal(t, eSel.(*indexSelector).index, rSel.(*indexSelector).index)
				return true
			case *arraySliceSelector:
				require.Equal(t, eSel.(*arraySliceSelector).start, rSel.(*arraySliceSelector).start)
				require.Equal(t, eSel.(*arraySliceSelector).end, rSel.(*arraySliceSelector).end)
				require.Equal(t, eSel.(*arraySliceSelector).step, rSel.(*arraySliceSelector).step)
				return true
			case *filterSelector:
				require.NotNil(t, rSel.(*filterSelector).expr, "nil found in filterSelector.arg")
				return assertEqualFilterExpr(t, eSel.(*filterSelector).expr, rSel.(*filterSelector).expr)
			default:
				require.Failf(t, "unknown selector type", "selType: %#v", eSel)
				return false
			}
		}, "failed:\neSel: %#v\nrSel: %#v\n", eSel, rSel)
	}
	return true
}

func assertEqualFilterExpr(t *testing.T, e filterExpr, r filterExpr) bool {
	if e == nil && r == nil {
		return true
	}
	require.Truef(t, r != nil, "found nil for filterExpr where expected: %v", e)
	require.Equal(t, e.getType(), r.getType(), "types of filterExpr do not match")
	require.Conditionf(t, func() bool {
		switch e.getType() {
		case logicalExprType:
			require.Equal(t, e.(*logicalExpr).logicalOp, r.(*logicalExpr).logicalOp, "logicalExpr.logicalOp do not match")
			return assertEqualFilterExpr(t, e.(*logicalExpr).left, r.(*logicalExpr).left) &&
				assertEqualFilterExpr(t, e.(*logicalExpr).right, r.(*logicalExpr).right)
		case compareExprType:
			require.Equal(t, e.(*compareExpr).compareOp, r.(*compareExpr).compareOp, "logicalExpr.compareOp do not match")
			return assertEqualFilterExpr(t, e.(*compareExpr).left, r.(*compareExpr).left) &&
				assertEqualFilterExpr(t, e.(*compareExpr).right, r.(*compareExpr).right)
		case filterQryType:
			require.Equal(t, e.(*filterQry).evalExistenceOnly, r.(*filterQry).evalExistenceOnly, "filterQry.evalExistenceOnly do not match")
			require.NotNil(t, r.(*filterQry).parser, "found nil in filterQry.qryParser")
			return assertEqualJPParser(t, e.(*filterQry).parser, r.(*filterQry).parser)
		case functionExprType:
			require.Equal(t, e.(*functionExpr).fct, r.(*functionExpr).fct, "functionExpr.fct do not match")
			require.NotNil(t, r.(*functionExpr).args, "found nil in functionExp.args")
			require.Equal(t, len(e.(*functionExpr).args), len(r.(*functionExpr).args), "len(functionExpr.args)")
			for i, ea := range e.(*functionExpr).args {
				assertEqualFilterExpr(t, ea, r.(*functionExpr).args[i])
			}
			return true
		case parenExprType:
			require.NotNil(t, e.(*parenExpr).inner, "nil found in parenExpr")
			return assertEqualFilterExpr(t, e.(*parenExpr).inner, r.(*parenExpr).inner)
		case stringLiteralType:
			require.Equal(t, e.(*stringLiteral).val, r.(*stringLiteral).val, "values of stringLiterals to do not match")
			return true
		case intLiteralType:
			require.Equal(t, e.(*intLiteral).val, r.(*intLiteral).val, "values of intLiterals to do not match")
			return true
		case floatLiteralType:
			require.Equal(t, e.(*floatLiteral).val, r.(*floatLiteral).val, "values of floatLiterals to do not match")
			return true
		case boolLiteralType:
			require.Equal(t, e.(*boolLiteral).val, r.(*boolLiteral).val, "values of boolLiterals to do not match")
			return true
		case nullLiteralType:
			return true
		default:
			require.Failf(t, "unknown filterExpr type", "exprType: %d", e.getType())
			return false
		}
	}, "filterExpr not equal")
	return true
}

type failParseQueryTest struct {
	name string
	text string
	err  string
}

func TestFailParseQuery(t *testing.T) {
	failTests := []failParseQueryTest{
		{"unclosed segment", "[hello", "unclosed segment"},
		{"child '.' operator lacking following selector", "@.", "missing selector"},
		{"descendant '..' operator lacking following selector", "$..", "missing selector"},
		{"missing end quote", "$.'bla", "missing end quote"},
		{"newline not supported in quoted string - even not when escaped", "$.'\\\n'", "escaping of newline not supported"},
		{"empty query", "", "empty query"},
		{"invalid compare op '='", "..[?.'color'='brown']", "invalid compare-operator - single '='"},
		{"singular-query required but not provided", "$[?.'persons'.* <= 'abc']", "expected singular-query for comparison-element"},
		{"chained comparisons not allowed", "$[?.'persons'[0].'firstName' <= 'abc' == true]", "chained comparison not allowed"},
		{"superrecurfields", "............................................................'Price'", "invalid chaining of '.'/'..' segment-operators"},
		{"invalid-fctname non-alphanumeric", "[?abc-def(.'abs')]", "invalid/non-alphanumeric function name"},
		{"invalid syntax '<X>.[<selector(s)>]' ", "$.['abs']", "syntax error"},
	}
	for _, test := range failTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newQueryParser()
			parser.name = test.name
			err := parser.parse(test.text)
			require.NotNilf(subT, err, "expected error %v", test.err)
			println("\n")
		})
	}
}
