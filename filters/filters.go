package filters

import (
	"fmt"

	"github.com/google/gopacket/layers"
)

// creates a filter that's true if the message is a SIP request
func filterRequest(args []sexp) (Filter, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("request takes no args, got %v: %w", args, ErrWrongArgCount)
	}
	return func(msg *layers.SIP) bool {
		return !msg.IsResponse
	}, nil
}

// creates a filter that's true if the sip message is a response
func filterResponse(args []sexp) (Filter, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("response takes no args, got %v: %w", args, ErrWrongArgCount)
	}
	return func(msg *layers.SIP) bool {
		return msg.IsResponse
	}, nil
}

// creates a filter that's true if the sip message has one of the methods in
// the arguments
func filterMethods(args []sexp) (Filter, error) {
	methods := make([]layers.SIPMethod, len(args))

	// first pull out all our methods.  If we catch any that aren't known sip
	// methods, that's an error.
	for i, a := range args {
		s := ""
		switch v := a.i.(type) {
		case qString:
			s = string(v)
		case string:
			s = v
		default:
			return nil, fmt.Errorf("arg type %v: %w", i, ErrMethodsType)
		}
		method, err := layers.GetSIPMethod(s)
		if err != nil {
			return nil, fmt.Errorf("bad argument (#%v), %v %w: %v", i, s, ErrMethodsType, err)
		}
		methods[i] = method
	}

	return func(msg *layers.SIP) bool {
		for _, m := range methods {
			if msg.Method == m {
				return true
			}
		}
		return false
	}, nil
}

// creates a filter that's true if the sip message has a certain status.
// filter returns false for all requests, since they have no status.
func filterStatus(args []sexp) (Filter, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("status needs 1 or more args: %w", ErrWrongArgCount)
	}

	// Pull out each code.  Any non-ints is an error.
	codes := make([]int, len(args))
	for i, a := range args {
		if n, ok := a.i.(int); ok {
			codes[i] = n
		} else {
			return nil, fmt.Errorf("%v: %w", a, ErrNeedInt)
		}
	}

	return func(msg *layers.SIP) bool {
		if !msg.IsResponse {
			return false
		}
		for _, c := range codes {
			if msg.ResponseCode == c {
				return true
			}
		}
		return false
	}, nil
}

// create a filter that's true if the sip message has a named header with any value.
func filterHasHeader(args []sexp) (Filter, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("hasheaders got [%v]: %w", args, ErrWrongArgCount)
	}
	h, ok := args[0].i.(qString)
	if !ok {
		return nil, fmt.Errorf("hasheaders: %w", ErrNeedString)
	}
	field := string(h)
	return func(msg *layers.SIP) bool {
		// Don't care what the value is, just that it's not empty.
		return msg.GetFirstHeader(field) != ""
	}, nil
}

// create a filter that returns true if the sip message has a header that
// matches a regexp.
func filterHeader(args []sexp) (Filter, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("headers got [%v]: %w", args, ErrWrongArgCount)
	}
	h, ok := args[0].i.(qString)
	if !ok {
		return nil, fmt.Errorf("hasheader first argument must be a quoted string: %w", ErrNeedString)
	}
	re, err := regexpString(args[1])
	if err != nil {
		return nil, fmt.Errorf("compiling header regexp: %w", err)
	}
	field := string(h)
	return func(msg *layers.SIP) bool {
		for _, h := range msg.GetHeader(field) {
			if re.MatchString(h) {
				return true
			}
		}
		return false
	}, nil
}

// create filter that returns true if the sip message has a "to" header that
// matches the regexp.
func filterTo(args []sexp) (Filter, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("to: %w", ErrWrongArgCount)
	}
	re, err := regexpString(args[0])
	if err != nil {
		return nil, fmt.Errorf("compiling to regexp: %w", err)
	}
	return func(msg *layers.SIP) bool {
		return re.MatchString(msg.GetTo())
	}, nil
}

// create filter that returns true if the sip message has a "from" header that
// matches the regexp.
func filterFrom(args []sexp) (Filter, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("to: %w", ErrWrongArgCount)
	}
	re, err := regexpString(args[0])
	if err != nil {
		return nil, fmt.Errorf("compiling from regexp: %w", err)
	}
	return func(msg *layers.SIP) bool {
		return re.MatchString(msg.GetFrom())
	}, nil
}

// create a filter that's true if any part of the message matches the regexp
func filterMessage(args []sexp) (Filter, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("message %v: %w", args, ErrWrongArgCount)
	}
	re, err := regexpString(args[0])
	if err != nil {
		return nil, fmt.Errorf("compiling regexp %v in message: %w", args[0].i, err)
	}
	return func(msg *layers.SIP) bool {
		return re.Match(msg.LayerContents()) || re.Match(msg.Payload())
	}, nil
}

// create a filter that's true if the message body (not headers) matches the
// regexp.
func filterBody(args []sexp) (Filter, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("body [%v]: %w", args, ErrWrongArgCount)
	}
	re, err := regexpString(args[0])
	if err != nil {
		return nil, fmt.Errorf("compiling regexp %v in body: %w", args[0].i, err)
	}
	return func(msg *layers.SIP) bool {
		return re.Match(msg.Payload())
	}, nil
}

// creates a filter which inverts truth value of the argument filter.
func filterNot(args []sexp) (Filter, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("not [%v]: %w", args, ErrWrongArgCount)
	}
	f, err := compileSexp(args[0])
	if err != nil {
		return nil, fmt.Errorf("compiling not filter: %w", err)
	}
	return func(msg *layers.SIP) bool {
		return !f(msg)
	}, nil
}

// Helper since any/all need to do the same checking.
func checkFilterArgs(name string, args []sexp) ([]Filter, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("%v got [%v]: %w", name, args, ErrWrongArgCount)
	}
	filters := []Filter{}
	for i, a := range args {
		f, err := compileSexp(a)
		if err != nil {
			return nil, fmt.Errorf("compiling %v filter, arg %d [%v]: %w", name, i+1, a, err)
		}
		filters = append(filters, f)
	}
	return filters, nil
}

// create a filter that's true if any one of all the arguments is true.
func filterAny(args []sexp) (Filter, error) {
	filters, err := checkFilterArgs("any", args)
	if err != nil {
		return nil, err
	}
	return func(msg *layers.SIP) bool {
		for _, f := range filters {
			if f(msg) {
				return true
			}
		}
		return false
	}, nil
}

// create filter that's true only if all the arguments are true.
func filterAll(args []sexp) (Filter, error) {
	filters, err := checkFilterArgs("all", args)
	if err != nil {
		return nil, err
	}
	return func(msg *layers.SIP) bool {
		for _, f := range filters {
			if !f(msg) {
				return false
			}
		}
		return true
	}, nil
}

// a filter function which always passes (used if filter source is empty).
func passFunc(*layers.SIP) bool {
	return true
}
