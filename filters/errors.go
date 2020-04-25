package filters

type constErr string

func (e constErr) Error() string { return string(e) }

const (
	// ErrMismatchedParen indicates a closing paren is missing somewhere.
	ErrMismatchedParen = constErr("unmatched parens")
	// ErrMismatchedQuote indicates a closing double quote on a string is missing.
	ErrMismatchedQuote = constErr("unmatched quote")
	// ErrNeedRegexp indicates the filter that takes a regexp string argument
	// didn't get one.
	ErrNeedRegexp = constErr("not a regexp")
	// ErrNeedInt indicates status function got a non-integer argument.
	ErrNeedInt = constErr("not an integer")
	// ErrNeedString indicates a function
	ErrNeedString = constErr("not a string")
	// ErrMethodsType indicates the methods function received a non-sip method
	// name
	ErrMethodsType = constErr("methods takes a list of sip method names")
	// ErrWrongArgCount indicates the function received too few or too many
	// args.
	ErrWrongArgCount = constErr("wrong number of args")
	// ErrExtraTokens indicates extra text after a fuction; use any/all to
	// chain multiple functions.
	ErrExtraTokens = constErr("unexpected token")
	// ErrUnknownFunc indicates a an attempt to use an unknown/unimplemented
	// filter function.
	ErrUnknownFunc = constErr("unknown filter function")
	// ErrEmptyExpression indicates an empty sub-expression was found.
	ErrEmptyExpression = constErr("empty expression")
	// ErrExpressionType indicates a function sub-expression started with an
	// int, quoted string, or other non-function name.
	ErrExpressionType = constErr("invalid expression initial type")
	// ErrBadRegexp indicates the argument given failed to successfully compile via regexp.Compile
	ErrBadRegexp = constErr("unable to compile regexp")
)
