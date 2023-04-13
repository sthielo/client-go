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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"regexp"
	"testing"
)

type parseTmplTest struct {
	name          string
	text          string
	templateElems []templateElem
}

var parseTmplTests = []parseTmplTest{
	{"plain", `hello jsonpath`, []templateElem{&textTemplateElem{"hello jsonpath"}}},

	{"variable", `hello {.'jsonpath'}`, []templateElem{
		&textTemplateElem{"hello "},
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"jsonpath"},
			}},
		}}, true, dummyInnerParser}},
	}},

	{"arrayfiled", `hello {['jsonpath']}`, []templateElem{
		&textTemplateElem{"hello "},
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"jsonpath"},
			}},
		}}, true, dummyInnerParser}},
	}},

	{"quote", `{"{"}`, []templateElem{&textTemplateElem{"{"}}},

	{"array", `{[1:3]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&arraySliceSelector{optionalInt{true, 1}, optionalInt{true, 3}, 1},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"allarray", `{.'book'[*].'author'}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"book"},
			}},
			&segmentImpl{childSegmentType, []selector{
				&wildcardSelector{},
			}},
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"author"},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"wildcard", `{.'bicycle'.*}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"bicycle"},
			}},
			&segmentImpl{childSegmentType, []selector{
				&wildcardSelector{},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"filter", `{[?(@.'price'<3)]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{&compareExpr{
					&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{&nameSelector{"price"}}},
					}}, true, dummyInnerParser}},
					&intLiteral{3},
					"<",
				}},
			}},
		}}, false, dummyInnerParser}},
	}},

	//{"recursive", `{..}`, []Node{newList(), newRecursive()}},		// TODO ??? '..' without subsequent '*' is invalid in jSONPath => release notes!!! change in behavior, throws an error!
	// TODO ... assume '..' not followed by anything as alias to '..*' - would actually match all children AND all of their descendants! => NO !!!
	// TODO ... assume '..' not followed by anything as alias to '*' - not '..*' !? - to stay compliant with previous implementation? is this really how previous implementation worked? no test really shows interpretation of pure '..' => NO

	{"recurField", `{..'price'}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{descendantSegmentType, []selector{
				&nameSelector{"price"},
			}},
		}}, false, dummyInnerParser}},
	}},

	// TODO probably not the intended JSONPath query - CORRECT(?): `{$['book']['price']}` or simply `{.book.price}`
	//{"arraydict", `{['book.price']}`, []Node{newList(),},
	{"arraydict", `{.'book'.'price'}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"book"},
			}},
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"price"},
			}},
		}}, true, dummyInnerParser}},
	}},

	// TODO doable with nested templates? e.g. `{.bicycle.price}{[3]}{.book.price}`
	//{"union", `{['bicycle.price', 3, 'book.price']}`, },
	{"union", `{.'bicycle'.'price'}{[3]}{.'book'.'price'}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"bicycle"},
			}},
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"price"},
			}},
		}}, true, dummyInnerParser}},
		&jsonpathTemplateElem{&queryParser{"tmplQry-1", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&indexSelector{3},
			}},
		}}, true, dummyInnerParser}},
		&jsonpathTemplateElem{&queryParser{"tmplQry-2", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"book"},
			}},
			&segmentImpl{childSegmentType, []selector{
				&nameSelector{"price"},
			}},
		}}, true, dummyInnerParser}},
	}},

	// TODO example from https://kubernetes.io/docs/reference/kubectl/jsonpath/ ... not valid JSONPath!
	// POSSIBLE [partial] SOLUTION?: `{[items][*][metadata, status][name, capacity]}`
	//   ==> assumes 'metadata' nod has no child 'capacity' and vice versa for 'status' not having a field/child 'name' => would work when allowMissingFields
	// BETTER using template range-end-op: `{range .items.*}{.metadata.name}{@.status.capacity}{end}`
	// {exampleFromDoc", `{.items[*]['metadata.name', 'status.capacity']}`, }
	{"exampleFromDoc", `{range .'items'.*}{@.'metadata'.'name'}{.'status'.'capacity'}{end}`, []templateElem{
		&rangeTemplateElem{
			&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
				&segmentImpl{childSegmentType, []selector{
					&nameSelector{"items"},
				}},
				&segmentImpl{childSegmentType, []selector{
					&wildcardSelector{},
				}},
			}}, false, dummyInnerParser},
			[]templateElem{
				&jsonpathTemplateElem{&queryParser{"tmplQry-1", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"metadata"},
					}},
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"name"},
					}},
				}}, true, dummyInnerParser}},
				&jsonpathTemplateElem{&queryParser{"tmplQry-2", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"status"},
					}},
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"capacity"},
					}},
				}}, true, dummyInnerParser}},
			},
		},
	}},

	{"range", `{range .'items'}{.'name'} , {end}`, []templateElem{
		&rangeTemplateElem{
			&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
				&segmentImpl{childSegmentType, []selector{
					&nameSelector{"items"},
				}},
			}}, true, dummyInnerParser},
			[]templateElem{
				&jsonpathTemplateElem{&queryParser{"tmplQry-1", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{
						&nameSelector{"name"},
					}},
				}}, true, dummyInnerParser}},
				&textTemplateElem{" , "},
			},
		},
	}},

	{"paired parentheses in quotes", `{[?(@.'status'.'nodeInfo'.'osImage' == "()")]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"status"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"nodeInfo"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"osImage"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"()"},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"paired parentheses in double quotes and with double quotes escape", `{[?(@.'status'.'nodeInfo'.'osImage' == "(\"\")")]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"status"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"nodeInfo"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"osImage"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"(\"\")"},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"irregular parentheses in double quotes", `{[?(@.'test' == "())(")]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"test"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"())("},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"plain text in single quotes", `{[?(@.'status'.'nodeInfo'.'osImage' == 'Linux')]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"status"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"nodeInfo"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"osImage"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"Linux"},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"test filter suffix", `{[?(@.'status'.'nodeInfo'.'osImage' == "{[()]}")]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"status"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"nodeInfo"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"osImage"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"{[()]}"},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"double inside single", `{[?(@.'status'.'nodeInfo'.'osImage' == "''")]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"status"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"nodeInfo"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"osImage"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"''"},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"single inside double", `{[?(@.'status'.'nodeInfo'.'osImage' == '""')]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"status"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"nodeInfo"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"osImage"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"\"\""},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"single containing escaped single", `{[?(@.'status'.'nodeInfo'.'osImage' == '\\\'')]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&filterSelector{
					&compareExpr{
						&filterQry{false, &queryParser{"filterQry-0", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"status"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"nodeInfo"},
							}},
							&segmentImpl{childSegmentType, []selector{
								&nameSelector{"osImage"},
							}},
						}}, true, dummyInnerParser}},
						&stringLiteral{"\\'"},
						"==",
					},
				},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"negative index slice, equals a[len-5] to a[len-1]", `{[-5:]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&arraySliceSelector{optionalInt{true, -5}, optionalInt{false, 0}, 1},
			}},
		}}, false, dummyInnerParser}},
	}},

	{"negative index slice, equals a[len-1]", `{[-1]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&indexSelector{-1},
			}},
		}}, true, dummyInnerParser}},
	}},

	{"negative index slice, equals a[1] to a[len-1]", `{[1:-1]}`, []templateElem{
		&jsonpathTemplateElem{&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
			&segmentImpl{childSegmentType, []selector{
				&arraySliceSelector{optionalInt{true, 1}, optionalInt{true, -1}, 1},
			}},
		}}, false, dummyInnerParser}},
	}},
	{"nested range-end-ops", `{range .items[*]}{.metadata.name}` + "{\"\t\"}" + `{range @.spec.containers[*]}{.name}{" "}{end}` + "{\"\t\"}" + `{range @.spec.containers[*]}{.name}` + "{\" \"}" + `{end}` + "{\"\n\"}" + `{end}`, []templateElem{
		&rangeTemplateElem{
			&queryParser{"tmplQry-0", &nodeIdentifier{rootNodeSymbol, []segment{
				&segmentImpl{childSegmentType, []selector{&nameSelector{"items"}}},
				&segmentImpl{childSegmentType, []selector{&wildcardSelector{}}},
			}}, false, dummyInnerParser},
			[]templateElem{
				&jsonpathTemplateElem{&queryParser{"tmplQry-1", &nodeIdentifier{currentNodeSymbol, []segment{
					&segmentImpl{childSegmentType, []selector{&nameSelector{"metadata"}}},
					&segmentImpl{childSegmentType, []selector{&nameSelector{"name"}}},
				}}, true, dummyInnerParser}},
				&textTemplateElem{"\t"},
				&rangeTemplateElem{
					&queryParser{"tmplQry-2", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{&nameSelector{"spec"}}},
						&segmentImpl{childSegmentType, []selector{&nameSelector{"containers"}}},
						&segmentImpl{childSegmentType, []selector{&wildcardSelector{}}},
					}}, false, dummyInnerParser},
					[]templateElem{
						&jsonpathTemplateElem{&queryParser{"tmplQry-3", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{&nameSelector{"name"}}},
						}}, true, dummyInnerParser}},
						&textTemplateElem{" "},
					},
				},
				&textTemplateElem{"\t"},
				&rangeTemplateElem{
					&queryParser{"tmplQry-4", &nodeIdentifier{currentNodeSymbol, []segment{
						&segmentImpl{childSegmentType, []selector{&nameSelector{"spec"}}},
						&segmentImpl{childSegmentType, []selector{&nameSelector{"containers"}}},
						&segmentImpl{childSegmentType, []selector{&wildcardSelector{}}},
					}}, false, dummyInnerParser},
					[]templateElem{
						&jsonpathTemplateElem{&queryParser{"tmplQry-5", &nodeIdentifier{currentNodeSymbol, []segment{
							&segmentImpl{childSegmentType, []selector{&nameSelector{"name"}}},
						}}, true, dummyInnerParser}},
						&textTemplateElem{" "},
					},
				},
				&textTemplateElem{"\n"},
			},
		},
	}},
}

func TestTemplateParser(t *testing.T) {
	for _, test := range parseTmplTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newTemplateParser(test.name)
			parser.Name = test.name
			err := parser.parse(test.text)
			require.Nilf(subT, err, "unexpected error %v", err)
			require.Conditionf(subT, func() bool {
				return assertEqualTmplElems(subT, test.templateElems, parser.templateElems)
			}, "templateElems not equal:\ne: %#v\nr: %#v\n", test.templateElems, parser.templateElems)
			println("\n")
		})
	}
}

func assertEqualTmplElems(t *testing.T, e []templateElem, r []templateElem) bool {
	require.Equal(t, len(e), len(r), "len([]templateElem) do not match")
	for i, eElem := range e {
		rElem := r[i]
		require.Equal(t, reflect.TypeOf(eElem), reflect.TypeOf(rElem), "types of templateElement[%d] do not match", i)
		require.Conditionf(t, func() bool {
			switch eElem.(type) {
			case *textTemplateElem:
				return assertEqualTextTmplElem(t, eElem.(*textTemplateElem), rElem.(*textTemplateElem))
			case *jsonpathTemplateElem:
				return assertEqualJPTmplElem(t, eElem.(*jsonpathTemplateElem), rElem.(*jsonpathTemplateElem))
			case *rangeTemplateElem:
				return assertEqualRangeTmplElem(t, eElem.(*rangeTemplateElem), rElem.(*rangeTemplateElem))
			default:
				require.Failf(t, "unknown segment type", "segType found: %T for %#v", eElem, eElem)
				return false
			}
		}, "elem did not match:\neElem: %#v\nrElem: %#v\n", eElem, rElem)
	}
	return true
}

func assertEqualTextTmplElem(t *testing.T, e *textTemplateElem, r *textTemplateElem) bool {
	require.Equal(t, e.text, r.text, "texts do not match")
	return true
}

func assertEqualJPTmplElem(t *testing.T, e *jsonpathTemplateElem, r *jsonpathTemplateElem) bool {
	require.NotNil(t, r.qryParser, "no jsonpathParser in testResult")
	return assertEqualJPParser(t, e.qryParser, r.qryParser)
}

func assertEqualRangeTmplElem(t *testing.T, e *rangeTemplateElem, r *rangeTemplateElem) bool {
	require.Conditionf(t, func() bool {
		return assertEqualJPParser(t, e.qryParser, r.qryParser)
	}, "range.qryParser not equal:\ne: %#v\nr: %#v\n", e.qryParser, r.qryParser)
	require.Conditionf(t, func() bool {
		return assertEqualTmplElems(t, e.elems, r.elems)
	}, "range.elems not equal:\ne: %#v\nr: %#v\n", e, r)
	return true
}

func assertEqualJPParser(t *testing.T, e *queryParser, r *queryParser) bool {
	require.Equal(t, e.name, r.name, ".name do not match")
	require.Equal(t, e.isSingular, r.isSingular, ".isSingular do not match")
	require.Equal(t, e.root.nodeIdentifierSymbol, r.root.nodeIdentifierSymbol, ".root.nodeIdentifierSymbol do not match")
	require.Conditionf(t, func() bool {
		return assertEqualJPSegs(t, e.root.segments, r.root.segments)
	}, "jsonpathparser did not match:\neSegs: %#v\nrSegs: %#v\n", e.root.segments, r.root.segments)
	return true
}

type failParseTmplTest struct {
	name string
	text string
	err  string
}

func TestFailParseTmpl(t *testing.T) {
	failTests := []failParseTmplTest{
		{"unclosed action", "{.hello", "unclosed action"},
		{"unterminated array", "{[1}", "unterminated array"},
		{"unterminated filter", "{[?(.price]}", "unterminated filter"},
		{"invalid multiple recursive descent", "{........}", "invalid multiple recursive descent"},
		{"invalid identifier", "{hello}", "unrecognized identifier hello"},
		{"invalid filter operator", "{.Book[?(@.Price<>10)]}", "unrecognized filter operator <>"},
		{"redundant end", "{range .Labels.*}{@}{end}{end}", "not in range, nothing to end"},
	}
	for _, test := range failTests {
		t.Run(test.name, func(subT *testing.T) {
			parser := newTemplateParser(test.name)
			parser.Name = test.name
			err := parser.parse(test.text)
			require.NotNilf(subT, err, "expected error", "%s", test.err)
			println("\n")
		})
	}
}

func TestRegexps(t *testing.T) {
	assert.True(t, rangeTmplElemStartRegexp().MatchString("{range"), "range start")
	assert.True(t, rangeTmplElemStartRegexp().MatchString("{ range"), "range start with whitespace")
	assert.True(t, rangeTmplElemStartRegexp().MatchString("{     range"), "range start with multiple whitespace")
	assert.True(t, rangeTmplElemStartRegexp().MatchString("{ \trange"), "range start with tab")

	assert.True(t, rangeTmplElemEndRegexp().MatchString("{end}"), "range end")
	assert.True(t, rangeTmplElemEndRegexp().MatchString("{ \t end}"), "range end with preceding whitespace")
	assert.True(t, rangeTmplElemEndRegexp().MatchString("{end \n }"), "range end followed by whitespace")
	assert.True(t, rangeTmplElemEndRegexp().MatchString("{ \tend \n}"), "range end with whitespaces on either side")

	assert.True(t, jpElemStartRegexp().MatchString("{$"), "abs jsonpath start")
	assert.True(t, jpElemStartRegexp().MatchString("{ \t $"), "abs jsonpath start with whitespaces")
	assert.True(t, jpElemStartRegexp().MatchString("{@"), "relative jsonpath start")
	assert.True(t, jpElemStartRegexp().MatchString("{\t \t@"), "relative jsonpath start with whitespaces")
	assert.True(t, jpElemStartRegexp().MatchString("{."), "dot jsonpath start")
	assert.True(t, jpElemStartRegexp().MatchString("{ \t \n."), "dot jsonpath start with whitespaces")
	assert.True(t, jpElemStartRegexp().MatchString("{["), "square bracket jsonpath start")
	assert.True(t, jpElemStartRegexp().MatchString("{\n["), "square bracket jsonpath start with whitespaces")

	assert.True(t, quotedElemStartRegexp().MatchString("{\""), "double quoted start")
	assert.True(t, quotedElemStartRegexp().MatchString("{   \""), "double quoted start with whitespaces")
	assert.True(t, quotedElemStartRegexp().MatchString("{'"), "single quoted start")
	assert.True(t, quotedElemStartRegexp().MatchString("{   '"), "single quoted start with whitespaces")

	r, _ := regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[f|F])$`, "%f")
	assert.True(t, r, "lower f")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%g")
	assert.True(t, r, "lower g")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%G")
	assert.True(t, r, "upper G")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%e")
	assert.True(t, r, "lower e")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%E")
	assert.True(t, r, "upper E")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%.2f")
	assert.True(t, r, "lower f with dec precision")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%3.f")
	assert.True(t, r, "lower f with width")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%3.2f")
	assert.True(t, r, "lower f with width and precision")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%3.2F")
	assert.True(t, r, "upper F with width and precision")

	r, _ = regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, "%F")
	assert.True(t, r, "upper F")
}
