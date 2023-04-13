package jsonpath

import (
	"fmt"
	"reflect"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// UnquoteExtend is almost same as strconv.Unquote(), but it supports parseNodeIdentifier single quotes as a string
func UnquoteExtend(s string) (string, error) {
	n := len(s)
	if n < 2 {
		return "", fmt.Errorf("quoted str too short")
	}
	quote := s[0]
	if quote != s[n-1] {
		return "", fmt.Errorf("start quote not matching end quote")
	}
	s = s[1 : n-1]

	if quote != '"' && quote != '\'' {
		return "", fmt.Errorf("expected single or double quotes")
	}

	// Is it trivial?  Avoid allocation.
	if !contains(s, '\\') && !contains(s, quote) {
		return s, nil
	}

	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(s)/2) // Try to avoid more allocations.
	for len(s) > 0 {
		c, multibyte, ss, err := strconv.UnquoteChar(s, quote)
		if err != nil {
			return "", err
		}
		s = ss
		if c < utf8.RuneSelf || !multibyte {
			buf = append(buf, byte(c))
		} else {
			n := utf8.EncodeRune(runeTmp[:], c)
			buf = append(buf, runeTmp[:n]...)
		}
	}
	return string(buf), nil
}

func contains(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}

func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() || v.IsZero() {
			return v, true
		}
	}
	return v, v.Kind() == reflect.Invalid
}
