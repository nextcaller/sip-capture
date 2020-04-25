package filters

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/gopacket/layers"
)

// Filter is a function which decides if a SIP message should pass or fail.
type Filter func(msg *layers.SIP) bool

type filterBuilders map[string]func([]sexp) (Filter, error)

// Compile a source in sexp format into an invokable  Filter
func Compile(source string) (Filter, error) {
	// Make sure our filter builders are initialized.
	if builders == nil {
		builders.init()
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return passFunc, nil
	}

	sexp, err := parseSexp(source)
	if err != nil {
		return nil, fmt.Errorf("filter parsing error: %w", err)
	}

	return compileSexp(sexp)
}

var (
	builders filterBuilders
)

// We can't initialize these as a package var, because recursion happens in
// some filters, and package level init can't handle that.
func (fb *filterBuilders) init() {
	*fb = map[string]func([]sexp) (Filter, error){
		"request":   filterRequest,
		"response":  filterResponse,
		"methods":   filterMethods,
		"status":    filterStatus,
		"hasheader": filterHasHeader,
		"to":        filterTo,
		"from":      filterFrom,
		"header":    filterHeader,
		"body":      filterBody,
		"message":   filterMessage,
		"not":       filterNot,
		"any":       filterAny,
		"all":       filterAll,
	}
}

// compile a function with possible argument list into a filter.  Some filter
// funcs recurse back to compileSexp in the case of embedded filters.
func compileSexp(s sexp) (Filter, error) {
	var args []sexp
	f := ""

	// Figure out if this is a bare string or a list type.  Anything else is an
	// error.
	switch v := s.i.(type) {
	case string:
		f = v
		args = []sexp{}
	case list:
		if len(v) < 1 {
			return nil, fmt.Errorf("expression [%v]: %w", v, ErrEmptyExpression)
		}
		var ok bool
		f, ok = v[0].i.(string)
		if !ok {
			return nil, fmt.Errorf("expression [%v] must start with a func name, not %v: %w", s, v[0], ErrExpressionType)
		}
		args = v[1:]
	default:
		return nil, fmt.Errorf("expression [%v] must start with func name, not %v: %w", s, v, ErrExpressionType)
	}

	// We now have a function name, exec its builder if we have one.
	if builder, ok := builders[f]; ok {
		return builder(args)
	}
	return nil, fmt.Errorf("%v: %w", f, ErrUnknownFunc)
}

// convenience func to convert an sexp that should contain a single quoted string
// into a filterable regexp.Regexp.
func regexpString(a sexp) (*regexp.Regexp, error) {
	s, ok := a.i.(qString)
	if !ok {
		return nil, ErrNeedString
	}
	re, err := regexp.Compile(string(s))
	if err != nil {
		return nil, fmt.Errorf("compiling regexp: %w: %v", ErrBadRegexp, err)
	}
	return re, nil
}
