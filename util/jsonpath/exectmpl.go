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
)

type tmplExecCtxt struct {
	out              io.Writer
	format           resultFormat
	rootDataNode     reflect.Value
	allowMissingKeys bool
	qryFunctions     functionRegistry
	enableDebugMsgs  bool
}

func (ctx tmplExecCtxt) isDebugEnabled() bool {
	return ctx.enableDebugMsgs
}

func executeTemplate(wr io.Writer, format resultFormat, parser *templateParser, rootDataNode reflect.Value, allowMissingKeys bool, queryFuncions functionRegistry, enableDebugMsgs bool) error {
	ctx := &tmplExecCtxt{wr, format, rootDataNode, allowMissingKeys, queryFuncions, enableDebugMsgs}
	for _, te := range parser.templateElems {
		err := execTmplElem(ctx, te, rootDataNode)
		if err != nil {
			return err
		}
	}
	return nil
}

func execTmplElem(ctx *tmplExecCtxt, te templateElem, curNode reflect.Value) error {
	// todo dbg msg entering and existing tmpl
	switch te.(type) {
	case *jsonpathTemplateElem:
		return execJPTmpl(ctx, te.(*jsonpathTemplateElem), curNode)
	case *rangeTemplateElem:
		return execRangeTmpl(ctx, te.(*rangeTemplateElem), curNode)
	case *textTemplateElem:
		return execTextTmpl(ctx, te.(*textTemplateElem))
	default:
		panic(fmt.Sprintf("unknown templateElem type: %T of %#v", te, te))
	}
}

func execTextTmpl(ctx *tmplExecCtxt, tt *textTemplateElem) error {
	_, err := fmt.Fprint(ctx.out, tt.text)
	if err != nil {
		// todo wrap with a common template execution error type
		return err
	}
	return nil
}

func execJPTmpl(ctx *tmplExecCtxt, jt *jsonpathTemplateElem, curNode reflect.Value) error {
	resultSet, err := executeQuery(jt.qryParser, ctx.rootDataNode, curNode, false, ctx.allowMissingKeys, ctx.qryFunctions, ctx.enableDebugMsgs)
	if err != nil {
		// todo wrap with a common template execution error type
		return err
	}
	if resultSet == nil {
		// todo wrap with a common template execution error type
		// todo debugMsg - was that intended
		return fmt.Errorf("nil returned by jsonpath query '%s' on data: %s", jt.qryParser.string(), ctx.rootDataNode)
	}
	prefix := ""
	switch ctx.format.format {
	case jsonFormatted:
		prefix = "\n"
	case legacyFormatted, condensedJsonFormatted:
		break
	default:
		panic(fmt.Sprintf("unknown format required: %d", ctx.format.format))
	}
	printResults(ctx.out, resultSet, ctx.format, prefix)
	return nil
}

func execRangeTmpl(ctx *tmplExecCtxt, rt *rangeTemplateElem, curNode reflect.Value) error {
	resultSet, err := executeQuery(rt.qryParser, ctx.rootDataNode, curNode, false, ctx.allowMissingKeys, ctx.qryFunctions, ctx.enableDebugMsgs)
	if err != nil {
		// todo wrap with a common template execution error type
		return err
	}
	if resultSet == nil {
		// todo wrap with a common template execution error type
		// todo debugMsg - was that intended
		return fmt.Errorf("nil returned by jsonpath query '%s' on data: %s", rt.qryParser.string(), ctx.rootDataNode)
	}

	for _, r := range resultSet.Elems {
		for _, t := range rt.elems {
			err := execTmplElem(ctx, t, r)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
