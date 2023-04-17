package jsonpath

import (
	"fmt"
	"reflect"
	"regexp"
)

type QueryResult interface {
	qryResultInheritanceLimiter()
}

// ResultSet represents the results of a filter-query
// OfSingularQuery indicates whether that query is a singular-query and as such guarantees max 1 element in Elems
type ResultSet struct {
	Elems []reflect.Value
}

func (_ ResultSet) qryResultInheritanceLimiter() {}

type Singular struct {
	Value interface{}
}

func (_ Singular) qryResultInheritanceLimiter() {}

// QueryFunction filter-functions gets either
// - ResultSet: a resultSet of a filter-query or another function (may be nil, empty and/or contain nil elements) or
// - Singular (containing bool, string, int, uint, float, array, map, struct): a singular value from a logical or comparison operator, another function or a singularQuery
// and can return either a
// - ResultSet: a resultSet (the same, a complete different one, a manipulated one, ...) or
// - Singular: a singular value
type QueryFunction func(arg ...QueryResult) (QueryResult, error)

// functions available within JSONPath query filters
// initialized with the default functions available
type functionRegistry map[string]QueryFunction

func newFunctionRegistry() functionRegistry {
	return map[string]QueryFunction{
		"count":  count,
		"length": length,
		"match":  match,
		"search": search,
	}
}

func getSingularString(r QueryResult) (*Singular, error) {
	if r == nil {
		return nil, nil
	}

	switch r.(type) {
	case *ResultSet:
		rs := r.(*ResultSet)
		// ... from singular-queries (or other queries that happen just to return max. ONE result) we might extract a singular string
		if len(rs.Elems) <= 0 {
			return nil, nil
		}
		if len(rs.Elems) > 1 {
			return nil, fmt.Errorf("cannot extract singular string from resultSet with multiple results: %#v", r)
		}
		reV, isNil := indirect(rs.Elems[0])
		if isNil {
			return nil, nil
		}
		switch reV.Kind() {
		case reflect.String:
			return &Singular{reV.String()}, nil
		default:
			// not defined other types
			return nil, nil
		}
	case *Singular: // defined for values of type array, map, struct, string
		rv := r.(*Singular).Value
		switch rv.(type) {
		case string:
			return &Singular{rv.(string)}, nil
		case reflect.Value:
			switch rv.(reflect.Value).Kind() {
			case reflect.String:
				return &Singular{rv.(reflect.Value).String()}, nil
			default:
				// not defined other types
				return nil, nil
			}
		default:
			// not defined for other simple types
			return nil, nil
		}
	default:
		panic(fmt.Sprintf("unknown type of QueryResult: %#v", r))
	}
}

func (r functionRegistry) register(name string, f QueryFunction) error {
	_, exists := r[name]
	if exists {
		return fmt.Errorf("function '%s' already defined", name)
	}
	r[name] = f
	return nil
}

// count returns the size of the ResultSet returning an int singular value
// returns nil in any other case (only defined for ResultSet and is therefore returning nil for any Singular)
func count(args ...QueryResult) (QueryResult, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("invalid nr of args to function 'length' - requires exactly ONE argument")
	}
	arg := args[0]
	if arg == nil {
		return nil, nil
	}
	switch arg.(type) {
	case *ResultSet: // the standard definition: size of resultSet
		rs := arg.(*ResultSet).Elems
		if rs == nil {
			return nil, nil
		}
		return &Singular{len(rs)}, nil
	// TODO shall we be nice and also give the size of singular values of type array, map, struct, string? => actually length() should be used in these cases!
	//case *Singular:
	//	rs := arg.(*Singular).Value
	//	switch rs.(type) {
	//	case reflect.Value:
	//		switch rs.(reflect.Value).Kind() {
	//		case reflect.Array, reflect.Slice, reflect.String, reflect.Map:
	//			return &Singular{rs.(reflect.Value).Len()}, nil
	//		case reflect.Struct:
	//			return &Singular{rs.(reflect.Value).NumField()}, nil
	//		default:
	//			// not defined other types - actually none expected here!!!
	//			return nil, nil
	//		}
	//	default:
	//		// not defined for simple types
	//		return nil, nil
	//	}
	default:
		return nil, fmt.Errorf("count - unsupported type of QueryResult: %#v", arg)
	}
}

// length returns the size of a singular-value of type array/slice, map, string, struct
// returns nil in any other case (only defined for Singular and results of singular-queries)
func length(args ...QueryResult) (QueryResult, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("invalid nr of args to function 'length' - requires exactly ONE argument")
	}
	arg := args[0]
	if nil == arg {
		return nil, nil
	}
	switch arg.(type) {
	case *ResultSet: // not defined for resultSets in general, ...
		rs := arg.(*ResultSet)
		// ... from singular-queries (or queries that happen to return a single result) we might extract a singular value to evaluate length for
		if len(rs.Elems) <= 0 {
			return nil, nil
		}
		if len(rs.Elems) > 1 {
			return nil, fmt.Errorf("cannot extract a singular value from a resultSet with multiple results: %#v", arg)
		}
		singularValue, isNil := indirect(rs.Elems[0])
		if isNil {
			return nil, nil
		}
		switch singularValue.Kind() {
		case reflect.Array, reflect.Slice, reflect.String, reflect.Map:
			return &Singular{singularValue.Len()}, nil
		case reflect.Struct:
			return &Singular{singularValue.NumField()}, nil
		default:
			// not defined for other types
			return nil, nil
		}
	case *Singular: // defined for values of type array, map, struct, string
		rs := arg.(*Singular).Value
		if rs == nil {
			return nil, nil
		}
		switch rs.(type) {
		case string:
			return &Singular{len(rs.(string))}, nil
		case reflect.Value:
			r, isNil := indirect(rs.(reflect.Value))
			if isNil {
				return nil, nil
			}
			switch r.Kind() {
			case reflect.Array, reflect.Slice, reflect.String, reflect.Map:
				return &Singular{r.Len()}, nil
			case reflect.Struct:
				return &Singular{r.NumField()}, nil
			default:
				// not defined for other types
				return nil, nil
			}
		default:
			// not defined for simple types
			return nil, nil
		}
	default:
		panic(fmt.Sprintf("unsupported type of QueryResult: %#v", arg))
	}
}

// match tests a Singular (or singular-query result) input argument with a regexp, a singular 2nd string arg (so string
// literal or singular-query returning a string), to return a singular bool value
// jsonpath regexp - actually using go regexp with the adaption described in this jsonpath regexp spec
// reference: https://www.ietf.org/archive/id/draft-ietf-jsonpath-iregexp-04.html#name-pcre-re2-ruby-regexps
// match vs search: according to spec: match must match the entire string
func match(args ...QueryResult) (QueryResult, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("invalid nr of args to function 'match' - requires exactly TWO arguments")
	}

	matchTargetSingular, err := getSingularString(args[0])
	if err != nil {
		return nil, err
	}

	regexpSingular, err := getSingularString(args[1])
	if err != nil {
		return nil, err
	}

	if matchTargetSingular == nil || regexpSingular == nil || matchTargetSingular.Value == nil || regexpSingular.Value == nil {
		return nil, nil
	}

	re, err := regexp.Compile("\\A(?:" + regexpSingular.Value.(string) + ")\\z")
	if err != nil {
		return nil, err
	}
	match := re.MatchString(matchTargetSingular.Value.(string))
	return &Singular{match}, nil
}

// search tests a Singular (or singular-query result) input argument with a regexp, a singular 2nd string arg (so string
// literal or singular-query returning a string), to return a singular bool value
// jsonpath regexp - actually using go regexp with the adaption described in this jsonpath regexp spec
// reference: https://www.ietf.org/archive/id/draft-ietf-jsonpath-iregexp-04.html#name-pcre-re2-ruby-regexps
// match vs search: according to spec: search is looking for a substraing that matches the regexp
func search(args ...QueryResult) (QueryResult, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("invalid nr of args to function 'match' - requires exactly TWO arguments")
	}

	matchTargetSingular, err := getSingularString(args[0])
	if err != nil {
		return nil, err
	}

	regexpSingular, err := getSingularString(args[1])
	if err != nil {
		return nil, err
	}

	if matchTargetSingular == nil || regexpSingular == nil || matchTargetSingular.Value == nil || regexpSingular.Value == nil {
		return nil, nil
	}

	re, err := regexp.Compile(regexpSingular.Value.(string))
	if err != nil {
		return nil, err
	}
	match := re.MatchString(matchTargetSingular.Value.(string))
	return &Singular{match}, nil
}
