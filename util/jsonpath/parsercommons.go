package jsonpath

import (
	"fmt"
	"regexp"
	"strconv"
	"unicode"
	"unicode/utf8"
)

const eof = -1

type innerParser struct {
	input string

	start int
	width int
	pos   int

	subQryCnt int
}

func (p *innerParser) next() rune {
	if p.pos >= len(p.input) {
		p.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(p.input[p.pos:])
	p.width = w
	p.pos += p.width
	return r
}

func (p *innerParser) nextSkippingAllWhitespaces() rune {
	for {
		r := p.next()
		switch r {
		case ' ', '\t', '\n', '\r':
			p.consume()
			break
		default:
			return r
		}
	}
}

func (p *innerParser) peekConsumingAllWhitespaces() rune {
	for {
		r := p.peek()
		switch r {
		case ' ', '\t', '\n', '\r':
			p.consumeNext()
			break
		default:
			return r
		}
	}
}

func (p *innerParser) lookAhead(test *regexp.Regexp) bool {
	return test.MatchString(p.input[p.pos:])
}

func (p *innerParser) peek() rune {
	if p.pos >= len(p.input) {
		p.width = 0
		return eof
	}
	r, _ := utf8.DecodeRuneInString(p.input[p.pos:])
	return r
}

// consume return the parsed text since last consume
func (p *innerParser) consume() string {
	value := p.input[p.start:p.pos]
	p.start = p.pos
	p.width = 0
	return value
}

func (p *innerParser) consumeNext() rune {
	r := p.next()
	p.consume()
	return r
}

// scan4DigitHex scans 4 digits without consuming
func (p *innerParser) scan4DigitHex() (string, error) {
	var result = ""
	for i := 0; i < 4; i++ {
		r := p.next()
		switch r {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A', 'a', 'B', 'b', 'C', 'c', 'D', 'd', 'E', 'e', 'F', 'f':
			result += string(r)
			break
		default:
			return "", fmt.Errorf("unexpected char/len of unicode hex value")
		}
	}
	return result, nil
}

// parseInteger parses an integer value with optional +/- sign prefixed
func (p *innerParser) parseInteger() (int, error) {
	switch r := p.peekConsumingAllWhitespaces(); {
	case r == '-' || r == '+' || unicode.IsDigit(r):
		p.next()
		break
	default:
		return 0, fmt.Errorf("unexpected char %c in number", r)
	}
Loop:
	for {
		switch r := p.peek(); {
		case unicode.IsDigit(r):
			p.next()
			break
		default:
			break Loop
		}
	}
	s := p.consume()
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid integer '%s' - %v", s, err)
	}
	return int(i), nil
}

// parseQuote parses and unquotes string inside double or single quote
func (p *innerParser) parseQuote() (string, error) {
	q := p.next() // ASSERT must be beginning quote '"' or '\''
Loop:
	for {
	Unescaped:
		switch p.next() {
		case eof:
			return "", fmt.Errorf("unterminated quoted string")
		case '\\': // escape char
			r := p.next()
			switch r {
			case '\n':
				return "", fmt.Errorf("newline not supported in quoted strings")
			case '\\', '/', q, '\b', '\r', '\f': // '/' can be escaped according to specs ?!?
				break Unescaped
			case 'U':
				_, err := p.scan4DigitHex()
				if err != nil {
					return "", err
				}
				// upper case: 8digits! ... so another 4!
			case 'u':
				_, err := p.scan4DigitHex()
				if err != nil {
					return "", err
				}
				break Unescaped
			default:
				return "", fmt.Errorf("unexpected escaping of char: %d (as string: %s)", r, string(r))
			}
		case q:
			break Loop
		}
	}
	value := p.consume()
	s, err := unquoteExtend(value)
	if err != nil {
		return "", err
	}
	return s, nil
}

func (p *innerParser) parseUnquoted(till rune) (string, error) {
	for {
		switch p.peek() {
		case till, eof:
			return p.consume(), nil
		case '\n', '\f', '\b', '\r':
			return "", fmt.Errorf("no escaped characters allowed in unquoted texts (hint: use quotes and proper escaping)")
		case '\'', '"':
			return "", fmt.Errorf("no quotes allowed in the middle of unquoted texts (hint: use quotes and proper escaping)")
		case '\\': // escape char
			// todo dbgMsg ... escape char in unquoted text interpreted as normal char!
			p.next()
			break
		default:
			p.next()
		}
	}
}

func (p *innerParser) unwrapByDelimiters(leftDelim rune, rightDelim rune, innerParser func() (interface{}, error), ignoreWhitespacesAdjacentToDelims bool) (interface{}, error) {
	if p.consumeNext() != leftDelim {
		return nil, fmt.Errorf("expected left delimiter '%s'", string(leftDelim))
	}
	if ignoreWhitespacesAdjacentToDelims {
		p.peekConsumingAllWhitespaces()
	}
	content, err := innerParser()
	if err != nil {
		return nil, err
	}
	var r rune
	if ignoreWhitespacesAdjacentToDelims {
		r = p.nextSkippingAllWhitespaces()
	} else {
		r = p.next()
	}
	if r != rightDelim {
		return nil, fmt.Errorf("expected right delimiter '%s'", string(rightDelim))
	}
	p.consume()
	return content, nil
}
