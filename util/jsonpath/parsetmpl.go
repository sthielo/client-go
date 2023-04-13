package jsonpath

import (
	"fmt"
	"regexp"
)

// templateParser allows parsing of JSONPath templates
type templateParser struct {
	Name          string
	p             *innerParser
	templateElems []templateElem
	absDefault    bool

	// internal usage
	tmplQryCnt int
}

// newTemplateParser allocates and returns a templateParser.
func newTemplateParser(name string) *templateParser {
	return &templateParser{
		Name:       name,
		p:          nil,
		absDefault: true,
	}
}

// parse initializes a templateParser with a JSONPath template by parsing and analyzing it
func (p *templateParser) parse(template string) error {
	p.p = &innerParser{template, 0, 0, 0, 0}
	err := p.parseTemplateElems()
	return err
}

func (p *templateParser) parseTemplateElems() error {
	p.templateElems = make([]templateElem, 0, 10)
	for {
		r := p.p.peek()
		switch r {
		case '"', '\'':
			text := p.p.consume()
			if len(text) > 0 {
				return SyntaxError{p.Name, "unexpected quote (hint: quotes may only be used at the beginning - not preceded by any whitespaces! - of a template element or must be escaped within a quoted text element)", p.p.input, p.p.pos}
			}
			elem, err := parseQuotedTextElem(p)
			if err != nil {
				return err
			}
			if elem != nil {
				p.templateElems = append(p.templateElems, elem)
			}
			break

		case '{': // start subquery or range elem or ...
			text := p.p.consume()
			if len(text) > 0 {
				p.templateElems = append(p.templateElems, &textTemplateElem{text})
			}

			elem, err := p.parseCurlyTemplateElem()
			if err != nil {
				return err
			}
			if elem != nil {
				p.templateElems = append(p.templateElems, elem)
			}
			break

		case eof: // end of template
			text := p.p.consume()
			if len(text) > 0 {
				p.templateElems = append(p.templateElems, &textTemplateElem{text})
			}
			return nil

		default: // assume text
			p.p.next()
			break
		}
	}
}

func rangeTmplElemStartRegexp() *regexp.Regexp {
	re, err := regexp.Compile("^\\{\\s*range")
	if err != nil {
		panic("fix regexp")
	}
	return re
}

func rangeTmplElemEndRegexp() *regexp.Regexp {
	re, err := regexp.Compile("^\\{\\s*end\\s*\\}")
	if err != nil {
		panic("fix regexp")
	}
	return re
}

func quotedElemStartRegexp() *regexp.Regexp {
	re, err := regexp.Compile("^\\{\\s*[\"']")
	if err != nil {
		panic("fix regexp")
	}
	return re
}

func jpElemStartRegexp() *regexp.Regexp {
	re, err := regexp.Compile("^\\{\\s*[$.@\\[]")
	if err != nil {
		panic("fix regexp")
	}
	return re
}

func (p *templateParser) parseCurlyTemplateElem() (templateElem, error) {
	if p.p.lookAhead(rangeTmplElemStartRegexp()) {
		return p.parseRangeEndOp()
	}

	switch {
	case p.p.lookAhead(quotedElemStartRegexp()): // quoted - 'static' - element
		return p.unwrapTemplateElemDelimiters(parseQuotedTextElem)

	case p.p.lookAhead(jpElemStartRegexp()):
		return p.unwrapTemplateElemDelimiters(parseJsonPathElem)

	default:
		return nil, SyntaxError{p.Name, "invalid template element (hint: static text elements need quotes)", p.p.input, p.p.pos}
		// todo interpret as unquoted textElem?
		//return p.parseCurlyUnquotedElem()
	}

	return nil, SyntaxError{p.Name, "found opening of template element '{', which does not contain a JSONPath expression, nor a quoted string, nor a range-end-operator starting tag. (hint1: '{' as part of a text requires quotes; hint2: text elements within curly brackets need to be quoted", p.p.input, p.p.pos}
}

func (p *templateParser) parseRangeEndOp() (*rangeTemplateElem, error) {
	unwrappedContent, err := p.unwrapTemplateElemDelimiters(parseRangeHeader)
	if err != nil {
		return nil, err
	}
	result := unwrappedContent.(*rangeTemplateElem)

	oldAbsDefault := p.absDefault
	p.absDefault = false
Loop:
	for {
		switch p.p.peek() {
		case '"', '\'':
			text := p.p.consume()
			if len(text) > 0 {
				return nil, SyntaxError{p.Name, "unescaped quotes can only be at the beginning of a quoted text element", p.p.input, p.p.pos}
			}
			textElem, err := parseQuotedTextElem(p)
			if err != nil {
				return nil, err
			}
			result.elems = append(result.elems, textElem)
			break

		case '{': // start subquery or 'end' ro just static [quoted] text
			text := p.p.consume()
			if len(text) > 0 {
				result.elems = append(result.elems, &textTemplateElem{text})
			}

			if p.p.lookAhead(rangeTmplElemEndRegexp()) {
				_, err := p.unwrapTemplateElemDelimiters(parseRangeEnd)
				if err != nil {
					return nil, err
				}
				break Loop // last range-end-operator element
			}

			elem, err := p.parseCurlyTemplateElem()
			if err != nil {
				return nil, err
			}
			if elem != nil {
				result.elems = append(result.elems, elem)
			}
			break

		case eof:
			return nil, SyntaxError{p.Name, "unexpected end of range-end-template", p.p.input, p.p.pos}

		default:
			p.p.next()
			break
		}
	}
	p.absDefault = oldAbsDefault
	return result, nil

}

func (p *templateParser) unwrapTemplateElemDelimiters(parseElem func(p *templateParser) (templateElem, error)) (templateElem, error) {
	result, err := p.p.unwrapByDelimiters('{', '}', func() (interface{}, error) {
		return parseElem(p)
	}, true)

	if err != nil {
		switch err.(type) {
		case SyntaxError:
			return nil, err
		default:
			return nil, SyntaxError{p.Name, err.Error(), p.p.input, p.p.pos}
		}
	}
	if result == nil {
		return nil, nil
	}
	return result.(templateElem), nil
}

func parseQuotedTextElem(p *templateParser) (templateElem, error) {
	text, err := p.p.parseQuote()
	if err != nil {
		return nil, SyntaxError{p.Name, err.Error(), p.p.input, p.p.pos}
	}
	if len(text) > 0 {
		return &textTemplateElem{text}, nil
	}
	return nil, nil
}

func parseJsonPathElem(p *templateParser) (templateElem, error) {
	p.tmplQryCnt++
	qryParser, err := parseInnerQuery(fmt.Sprintf("tmplQry-%d", p.tmplQryCnt-1), p.absDefault, p.p)
	if err != nil {
		return nil, err
	}
	return &jsonpathTemplateElem{qryParser}, nil
}

// parseRangeTemplate parses template of k8s JSONPath extension range-end-operator
func parseRangeHeader(p *templateParser) (templateElem, error) {
	result := &rangeTemplateElem{nil, make([]templateElem, 0, 10)}

	for _, _ = range "range" {
		p.p.next()
	}
	// ASSERT must be 'range'
	if "range" != p.p.consume() {
		panic("internal error - assertion")
	}

	qryElem, err := parseJsonPathElem(p)
	if err != nil {
		return nil, err
	}
	result.qryParser = qryElem.(*jsonpathTemplateElem).qryParser
	return result, nil
}

func parseRangeEnd(p *templateParser) (templateElem, error) {
	for _, _ = range "end" {
		p.p.next()
	}
	if "end" != p.p.consume() {
		panic("internal error - assertion")
	}

	// just to fit the unwrapper function's signature
	return nil, nil
}

func (p *templateParser) string() string {
	tmplElemsStr := ""
	for _, te := range p.templateElems {
		if te != nil {
			tmplElemsStr += te.string()
		}
	}
	return fmt.Sprintf("{name=%s}%s", p.Name, tmplElemsStr)
}

func (p *templateParser) parseCurlyUnquotedElem() (templateElem, error) {
	result, err := p.p.unwrapByDelimiters('{', '}', func() (interface{}, error) {
		return p.p.parseUnquoted('}')
	}, false)

	if err != nil {
		switch err.(type) {
		case SyntaxError:
			return nil, err
		default:
			return nil, SyntaxError{p.Name, err.Error(), p.p.input, p.p.pos}
		}
	}
	return &textTemplateElem{result.(string)}, nil

}
