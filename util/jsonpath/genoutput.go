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
	"io"
	"reflect"
	"strconv"
)

type resultFormat struct {
	format resultFormatType

	// floatFormat printf expression to be used for formatting floats
	// defaults to %g
	floatFormat string
}

type resultFormatType uint

const (
	// jsonFormatted unfolded, human readable format.
	jsonFormatted resultFormatType = iota

	// legacyFormatted to fit backward compatibility
	legacyFormatted

	// condensedJsonFormatted used e.g. FOR LEGACY within complex resultSetElem types
	condensedJsonFormatted
)

type prefixType uint

const (
	structureStart prefixType = iota
	structureElemSeparator
	structureEnd
	keyValueSeparator
)

var structurePrefixes = map[resultFormatType]map[reflect.Kind]map[prefixType]string{
	jsonFormatted: {
		reflect.Array: {
			structureStart:         "[",
			structureElemSeparator: ",",
			structureEnd:           "]",
		},
		reflect.Struct: {
			structureStart:         "{",
			structureElemSeparator: ",",
			structureEnd:           "}",
			keyValueSeparator:      ":",
		},
		reflect.Map: {
			structureStart:         "{",
			structureElemSeparator: ",",
			structureEnd:           "}",
			keyValueSeparator:      ":",
		},
		999: {
			structureStart:         "[",
			structureElemSeparator: ",",
			structureEnd:           "]",
		},
	},
	legacyFormatted: {
		reflect.Array: {
			structureStart:         "[",
			structureElemSeparator: ",", // space for backward compatibility
			structureEnd:           "]",
		},
		reflect.Struct: {
			structureStart:         "{",
			structureElemSeparator: ",",
			structureEnd:           "}",
			keyValueSeparator:      ":",
		},
		reflect.Map: {
			structureStart:         "{",
			structureElemSeparator: ",",
			structureEnd:           "}",
			keyValueSeparator:      ":",
		},
		999: {
			structureStart:         "",
			structureElemSeparator: " ",
			structureEnd:           "",
		},
	},
	condensedJsonFormatted: {
		reflect.Array: {
			structureStart:         "[",
			structureElemSeparator: ",",
			structureEnd:           "]",
		},
		reflect.Struct: {
			structureStart:         "{",
			structureElemSeparator: ",",
			structureEnd:           "}",
			keyValueSeparator:      ":",
		},
		reflect.Map: {
			structureStart:         "{",
			structureElemSeparator: ",",
			structureEnd:           "}",
			keyValueSeparator:      ":",
		},
		999: {
			structureStart:         "[",
			structureElemSeparator: ",",
			structureEnd:           "]",
		},
	},
}

const indent = "  "

// printResults writes the results into writer
func printResults(wr io.Writer, result QueryResult, format resultFormat, prefix string) {
	switch result.(type) {
	case *ResultSet:
		printResultSet(wr, result.(*ResultSet), format, prefix)
		return
	case *Singular:
		printValue(wr, result.(*Singular).Value, format, prefix)
	default:
		panic(fmt.Sprintf("unknown result type of: %#v", result))
	}
}
func printResultSet(wr io.Writer, resultSet *ResultSet, format resultFormat, prefix string) {
	if len(prefix) == 0 || prefix[0] != '\n' {
		prefix = "\n" + prefix
	}
	if resultSet == nil || resultSet.Elems == nil {
		_, err := fmt.Fprint(wr, "null")
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		return
	}

	l := len(resultSet.Elems)
	structurePrefix := printStructureHeader(wr, format, 999, l, prefix)
	for i := 0; i < l; i++ {
		printValue(wr, resultSet.Elems[i], format, structurePrefix)
		printStructureSeparator(wr, format, 999, i, l, structurePrefix)
	}
	printStructureFooter(wr, format, 999, l, structurePrefix)
}

func printValue(wr io.Writer, value interface{}, format resultFormat, prefix string) {
	if value == nil {
		_, err := fmt.Fprint(wr, "null")
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		return
	}
	v := value
	for {
		if v == nil {
			_, err := fmt.Fprint(wr, "null")
			if err != nil {
				panic(fmt.Sprintf("cannot write to output: %v", err))
			}
			return
		}
		switch v.(type) {
		case reflect.Value:
			vV, isNil := indirect(v.(reflect.Value))
			if isNil {
				_, err := fmt.Fprint(wr, "null")
				if err != nil {
					panic(fmt.Sprintf("cannot write to output: %v", err))
				}
				return
			}
			switch vV.Kind() {
			case reflect.Bool:
				_, err := fmt.Fprintf(wr, "%t", vV.Bool())
				if err != nil {
					panic(fmt.Sprintf("cannot write to output: %v", err))
				}
				return
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				_, err := fmt.Fprintf(wr, "%d", vV.Int())
				if err != nil {
					panic(fmt.Sprintf("cannot write to output: %v", err))
				}
				return
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				_, err := fmt.Fprintf(wr, "%d", vV.Uint())
				if err != nil {
					panic(fmt.Sprintf("cannot write to output: %v", err))
				}
				return
			case reflect.Float32, reflect.Float64:
				_, err := fmt.Fprintf(wr, format.floatFormat, vV.Float())
				if err != nil {
					panic(fmt.Sprintf("cannot write to output: %v", err))
				}
				return
			case reflect.Array, reflect.Slice:
				printArray(wr, vV, format, prefix)
				return
			case reflect.Map:
				printMap(wr, vV, format, prefix)
				return
			case reflect.Struct:
				printStruct(wr, vV, format, prefix)
				return
			case reflect.String:
				s := vV.String()
				s = strconv.Quote(s)
				switch format.format {
				case jsonFormatted, condensedJsonFormatted:
					break
				case legacyFormatted:
					// also use quoted version. already for security, not to have strange side effects
					// for control chars. BUT for backward compatibility at least support the probably
					// commonly used alphanumerics
					s = s[1 : len(s)-1] // remove quotes
					break
				default:
					panic(fmt.Sprintf("internal error - unknown format required: %d", format.format))
				}
				_, err := fmt.Fprint(wr, s)
				if err != nil {
					panic(fmt.Sprintf("cannot write to output: %v", err))
				}
				return
			default:
				panic(fmt.Sprintf("unsupported kind for a result value: %d of %#v", vV.Kind(), vV))
			}
		default:
			v = reflect.ValueOf(v)
			break // loop again with a reflect.Value representation of the value
		}
	}
}

func printStructureHeader(wr io.Writer, format resultFormat, structureType reflect.Kind, length int, prefix string) string {
	structurePrefix := prefix
	start := structurePrefixes[format.format][structureType][structureStart]
	switch format.format {
	case jsonFormatted:
		structurePrefix += indent
		_, err := fmt.Fprintf(wr, "%s%s", start, structurePrefix)
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		break
	case condensedJsonFormatted:
		_, err := fmt.Fprint(wr, start)
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		break
	case legacyFormatted:
		if structureType != reflect.Array || length > 1 {
			_, err := fmt.Fprint(wr, start)
			if err != nil {
				panic(fmt.Sprintf("cannot write to output: %v", err))
			}
		}
		break
	default:
		panic(fmt.Sprintf("unknown format required %d", format.format))
	}
	return structurePrefix
}

func printStructureSeparator(wr io.Writer, format resultFormat, structureType reflect.Kind, index int, length int, structurePrefix string) {
	if index < length-1 {
		separator := structurePrefixes[format.format][structureType][structureElemSeparator]
		switch format.format {
		case jsonFormatted:
			_, err := fmt.Fprintf(wr, "%s%s", separator, structurePrefix)
			if err != nil {
				panic(fmt.Sprintf("cannot write to output: %v", err))
			}
			break
		case condensedJsonFormatted, legacyFormatted:
			_, err := fmt.Fprint(wr, separator)
			if err != nil {
				panic(fmt.Sprintf("cannot write to output: %v", err))
			}
			break
		default:
			panic(fmt.Sprintf("unknown format required %d", format.format))
		}
	}
}

func printStructureFooter(wr io.Writer, format resultFormat, structureType reflect.Kind, length int, structurePrefix string) {
	end := structurePrefixes[format.format][structureType][structureEnd]
	switch format.format {
	case jsonFormatted:
		newPrefix := structurePrefix[:len(structurePrefix)-len(indent)]
		_, err := fmt.Fprintf(wr, "%s%s", newPrefix, end)
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		break
	case condensedJsonFormatted:
		_, err := fmt.Fprint(wr, end)
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		break
	case legacyFormatted:
		if structureType != reflect.Array || length > 1 {
			_, err := fmt.Fprint(wr, end)
			if err != nil {
				panic(fmt.Sprintf("cannot write to output: %v", err))
			}
		}
		break
	default:
		panic(fmt.Sprintf("unknown format required %d", format.format))
	}
}

func printStructureKeyValueSeparator(wr io.Writer, format resultFormat, structureType reflect.Kind) {
	separator := structurePrefixes[format.format][structureType][keyValueSeparator]
	_, err := fmt.Fprint(wr, separator)
	if err != nil {
		panic(fmt.Sprintf("cannot write to output: %v", err))
	}
}

func printArray(wr io.Writer, arrayVal reflect.Value, format resultFormat, prefix string) {
	v, isNil := indirect(arrayVal)
	if isNil {
		_, err := fmt.Fprint(wr, "null")
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		return
	}
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		panic(fmt.Sprintf("internal error - unexpected argument type: %d of %#v - expected reflect.Array or reflect.Slice", arrayVal.Kind(), arrayVal))
	}
	l := v.Len()
	nonSimpleTypeFormat := format
	if format.format == legacyFormatted {
		nonSimpleTypeFormat = resultFormat{condensedJsonFormatted, format.floatFormat}
	}
	structurePrefix := printStructureHeader(wr, nonSimpleTypeFormat, reflect.Array, l, prefix)
	for i := 0; i < l; i++ {
		printValue(wr, v.Index(i), nonSimpleTypeFormat, structurePrefix)
		printStructureSeparator(wr, nonSimpleTypeFormat, reflect.Array, i, l, structurePrefix)
	}
	printStructureFooter(wr, nonSimpleTypeFormat, reflect.Array, l, structurePrefix)
}

func printMap(wr io.Writer, mapVal reflect.Value, format resultFormat, prefix string) {
	v, isNil := indirect(mapVal)
	if isNil {
		_, err := fmt.Fprint(wr, "null")
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		return
	}
	if v.Kind() != reflect.Map {
		panic(fmt.Sprintf("internal error - unexpected argument type: %d of %#v - expected reflect.Map", mapVal.Kind(), mapVal))
	}
	l := v.Len()
	nonSimpleTypeFormat := format
	if format.format == legacyFormatted {
		nonSimpleTypeFormat = resultFormat{condensedJsonFormatted, format.floatFormat}
	}
	structurePrefix := printStructureHeader(wr, nonSimpleTypeFormat, reflect.Map, l, prefix)
	for i, k := range v.MapKeys() {
		f, _ := indirect(v.MapIndex(k))
		printValue(wr, k, nonSimpleTypeFormat, structurePrefix)
		printStructureKeyValueSeparator(wr, nonSimpleTypeFormat, reflect.Map)
		printValue(wr, f, nonSimpleTypeFormat, structurePrefix)
		printStructureSeparator(wr, nonSimpleTypeFormat, reflect.Map, i, l, structurePrefix)
	}
	printStructureFooter(wr, nonSimpleTypeFormat, reflect.Map, l, structurePrefix)
}

func printStruct(wr io.Writer, structVal reflect.Value, format resultFormat, prefix string) {
	v, isNil := indirect(structVal)
	if isNil {
		_, err := fmt.Fprint(wr, "null")
		if err != nil {
			panic(fmt.Sprintf("cannot write to output: %v", err))
		}
		return
	}
	if v.Kind() != reflect.Struct {
		panic(fmt.Sprintf("internal error - unexpected argument type: %d of %#v - expected reflect.Struct", structVal.Kind(), structVal))
	}
	l := v.NumField()
	nonSimpleTypeFormat := format
	if format.format == legacyFormatted {
		nonSimpleTypeFormat = resultFormat{condensedJsonFormatted, format.floatFormat}
	}
	structurePrefix := printStructureHeader(wr, nonSimpleTypeFormat, reflect.Struct, l, prefix)
	for i := 0; i < l; i++ {
		fieldName := v.Type().Field(i).Tag.Get("json")
		if len(fieldName) <= 0 {
			fieldName = v.Type().Field(i).Name
		}
		printValue(wr, fieldName, nonSimpleTypeFormat, structurePrefix)
		printStructureKeyValueSeparator(wr, nonSimpleTypeFormat, reflect.Struct)
		f, _ := indirect(v.Field(i))
		printValue(wr, f, nonSimpleTypeFormat, structurePrefix)
		printStructureSeparator(wr, nonSimpleTypeFormat, reflect.Struct, i, l, structurePrefix)
	}
	printStructureFooter(wr, nonSimpleTypeFormat, reflect.Struct, l, structurePrefix)
}
