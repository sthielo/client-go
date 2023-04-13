package jsonpath

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type charHistogram map[rune]uint

func charHistogramFrom(text string) charHistogram {
	h := make(map[rune]uint, 100)
	for _, r := range text {
		_, exists := h[r]
		if exists {
			h[r] += 1
		} else {
			h[r] = 1
		}
	}
	return h
}

func requireEqualCharHistogram(t *testing.T, expected string, text string, msgAndArgs ...interface{}) {
	expectedHistogram := charHistogramFrom(expected)
	textHistogram := charHistogramFrom(text)
	require.EqualValues(t, expectedHistogram, textHistogram, msgAndArgs)
}

func findNextExpectedInRemainingResults(t *testing.T, expected []interface{}, remainingResults []reflect.Value) {
	if len(expected) <= 0 {
		require.Equal(t, 0, len(remainingResults))
		return
	}
	e := expected[0]
	findNextExpectedInRemainingResults(t, expected[1:], findAndRemove(t, e, remainingResults))
}

func findAndRemove(t *testing.T, e interface{}, results []reflect.Value) []reflect.Value {
	eV := reflect.ValueOf(e)
	found := false
	remainingResults := make([]reflect.Value, 0, len(results))
	for _, r := range results {
		if !found && checkEqualValue(true, t, eV, r) {
			found = true
		} else {
			remainingResults = append(remainingResults, r)
		}
	}
	require.Truef(t, found, "expected '%#v' not found in remaining result set", e)
	return remainingResults
}

func requireExpectedString(t *testing.T, expected interface{}, actual string, expectOrderedResult bool) {
	switch expected.(type) {
	case int:
		fmt.Printf("\nexpected length: %d", expected.(int))
		require.Equal(t, expected.(int), len(actual), "result length not as expected")
		break
	case string:
		fmt.Printf("\nexpected: >>%s<<", expected.(string))
		require.Equal(t, len(expected.(string)), len(actual), "result length not as expected")
		if expectOrderedResult {
			require.Equal(t, expected.(string), actual, "result not as expected")
		} else {
			requireEqualCharHistogram(t, expected.(string), actual, "result's charHistogram not as expected (unordered result! => compare charHistogram)")
		}
		break
	default:
		panic("invalid 'expected': string or int (length) value expected")
	}
}
