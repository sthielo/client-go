package jsonpath

import (
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestCount(t *testing.T) {
	result, err := count(&ResultSet{[]reflect.Value{reflect.ValueOf("abc"), reflect.ValueOf(123)}})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{2}, result, "count result set")
}

func TestCountEmpty(t *testing.T) {
	result, err := count(&ResultSet{[]reflect.Value{}})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{0}, result, "count=0 for empty result set")
}
func TestCountSingular(t *testing.T) {
	_, err := count(&Singular{3})
	require.NotNil(t, err, "no count on singular")
}

func TestCountTooManyArgs(t *testing.T) {
	_, err := count(&ResultSet{make([]reflect.Value, 0, 0)}, &Singular{"abc"})
	require.NotNil(t, err, "too many args")
}

func TestCountTooFewArgs(t *testing.T) {
	_, err := count()
	require.NotNil(t, err, "missing arg")
}

func TestLengthStringSingular(t *testing.T) {
	result, err := length(&Singular{"abc"})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{3}, result, "length of string as singular result in set")
}

func TestLengthStringOneResultInSet(t *testing.T) {
	result, err := length(&ResultSet{[]reflect.Value{reflect.ValueOf("abc")}})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{3}, result, "length of string as singular result in set")
}

func TestLengthArray(t *testing.T) {
	result, err := length(&ResultSet{[]reflect.Value{reflect.ValueOf([]string{"a", "bc", "d"})}})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{3}, result, "length of array")
}

func TestLengthMap(t *testing.T) {
	result, err := length(&ResultSet{[]reflect.Value{reflect.ValueOf(map[string]string{"a": "a", "bc": "b", "d": "z"})}})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{3}, result, "length of map")
}

type x struct {
	a  string
	bc int
	d  float32
}

func TestLengthStruct(t *testing.T) {
	result, err := length(&ResultSet{[]reflect.Value{reflect.ValueOf(x{"a", 1, 1.02})}})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{3}, result, "length of struct")
}

func TestLengthIntSingular(t *testing.T) {
	result, err := length(&Singular{1})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Nil(t, result, "int has no length")
}

func TestLengthEmpty(t *testing.T) {
	result, err := length(&ResultSet{[]reflect.Value{}})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Nil(t, result, "nothing to take length of")
}
func TestLengthTooManyArgs(t *testing.T) {
	_, err := length(&ResultSet{make([]reflect.Value, 0, 0)}, &Singular{"abc"})
	require.NotNil(t, err, "too many args")
}

func TestLengthTooFewArgs(t *testing.T) {
	_, err := length()
	require.NotNil(t, err, "missing arg")
}

func TestMatch(t *testing.T) {
	result, err := match(&Singular{"abbbbbc"}, &Singular{"ab+c"})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{true}, result, "matches")
}

func TestMatchFailSubstringonly(t *testing.T) {
	result, err := match(&Singular{"abbbbbc"}, &Singular{"b+"})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{false}, result, "no match on substrings")
}

func TestSearch(t *testing.T) {
	result, err := search(&Singular{"abbbbbc"}, &Singular{"b+"})
	require.Nilf(t, err, "unexpected error: %#v", err)
	require.Equal(t, &Singular{true}, result, "find substring that matches")
}
