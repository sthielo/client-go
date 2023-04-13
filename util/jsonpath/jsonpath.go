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
	"regexp"
)

type JSONPath struct {
	name   string
	parser *templateParser

	allowMissingKeys bool
	outputFormat     resultFormat

	qryFilterFunctions functionRegistry
	enableDebugMsgs    bool
}

// for backward compatibility
func New(name string) *JSONPath {
	return &JSONPath{
		name:               name,
		qryFilterFunctions: newFunctionRegistry(),
		outputFormat:       resultFormat{legacyFormatted, "%g"},
	}
}

// for backward compatibility
func (j *JSONPath) Parse(text string) error {
	// overwrite already parsed template
	err := j.parse(text)
	if err != nil {
		return err
	}
	return err
}

// NewJSONPath creates a new JSONPath with the given name AND parses the template given
func NewJSONPath(name string, jsonPathTemplate string) (*JSONPath, error) {
	j := &JSONPath{
		name:               name,
		qryFilterFunctions: newFunctionRegistry(),
		outputFormat:       resultFormat{legacyFormatted, "%g"},
		allowMissingKeys:   true,
	}
	err := j.parse(jsonPathTemplate)
	if err != nil {
		return nil, err
	}
	return j, err
}

// AllowMissingKeys allows a caller to specify whether they want an error if a field or map key
// cannot be located, or simply an empty result. The receiver is returned for chaining.
func (j *JSONPath) AllowMissingKeys(allow bool) *JSONPath {
	j.allowMissingKeys = allow
	return j
}

// EnableJSONOutput changes the printResults behavior to return a JSON array of results
func (j *JSONPath) EnableJSONOutput(v bool) *JSONPath {
	j.outputFormat = resultFormat{jsonFormatted, j.outputFormat.floatFormat}
	return j
}

// SetFloatFormat defines printf-style format to be used for floats to generate output
// default to "%g"
func (j *JSONPath) SetFloatFormat(floatFormat string) *JSONPath {
	validFloatFormat, _ := regexp.MatchString(`^%(e|E|g|G|(\d*\.\d*)?[fF])$`, floatFormat)
	if !validFloatFormat {
		panic("illegal float format - use printf style")
	}
	j.outputFormat.floatFormat = floatFormat
	return j
}

func (j *JSONPath) RegisterFilterFunction(name string, f QueryFunction) error {
	return j.qryFilterFunctions.register(name, f)
}

func (j *JSONPath) EnableDebugMsgs() {
	j.enableDebugMsgs = true
}

// Execute binds data into JSONPath and writes the result.
func (j *JSONPath) Execute(wr io.Writer, data interface{}) error {
	parser := j.parser
	if parser == nil {
		return fmt.Errorf("%s is an incomplete JSONPath Template - needs to be parsed first", j.name)
	}
	return executeTemplate(wr, j.outputFormat, parser, reflect.ValueOf(data), j.allowMissingKeys, j.qryFilterFunctions, j.enableDebugMsgs)
}

// newTemplateParser parses the given template and returns an error.
func (j *JSONPath) parse(jsonPathTemplate string) error {
	j.parser = newTemplateParser("JSONPathTemplateParser")
	return j.parser.parse(jsonPathTemplate)
}
