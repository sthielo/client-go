package jsonpath

import (
	"fmt"
	"strings"
)

type SyntaxError struct {
	parserName string
	msg        string
	input      string
	pos        int
}

func (e SyntaxError) Error() string {
	posMarker := strings.Repeat(" ", e.pos) + "^"
	return fmt.Sprintf("qryParser '%s' - syntax error (at pos %d): %s\n%q\n%s", e.parserName, e.pos, e.msg, e.input, posMarker)
}

type ExecutionError struct {
	parserName string
	msg        string
}

func (e ExecutionError) Error() string {
	return fmt.Sprintf("execution of '%s' - execution error: %s", e.parserName, e.msg)
}
