/*
Package filters implements a SIP message matching filter s-expression DSL.

The syntax of the DSL is an s-expressions, selecting which aspects of the SIP
message to match against.   The input to the resulting Filter is in the form of
github.com/google/gopacket *layers.SIP structs.

The simplest filter is the empty string, which returns a Filter function that
will match any possible *layers.SIP message.

The following SIP selection functions are available:
	request		is a SIP request
	response		is a SIP response
	(status n ...)	is a SIP response with any of the numeric status codes.
	(method s ...)	has one of the listed SIP methods.
	(hasheader s)	has any header with the given name
	(header s re)	has the given header with a value that matches a regexp
	(body re)		the body matches a regexp
	(message re)	anywhere in the whole message matches a regexp

Additionally, the following three logic functions can be used to build
complex filter functions:
	(all f ...)	each given filter is true
	(any f ...)	at least one given filter is true
	(not f)	the given filter's output is negated

The rule compiler parses lisp-like s-expressions, ignoring whitespace.  String
and regular expression arguments are quoted with "double quotes", while number
arguments may be simple integers.  If any part of the expression cannot be
interpreted, or a regular expression fails to compile, then Compile() will
return an error.

Function descriptions:

	request - Returns a match if the SIP message is the request side of a
	transaction.

	response - Returns a match if the SIP message is the response side of a
	transaction.  It is exclusive with request(); no message will pass both
	request() and response().

	(status n ...) - Returns a match if SIP message is a response, and the
	message's response code matches one of the arguments given.  Arguments must
	be integer numbers.

	(method s ...) - Returns a match if the SIP message's method matches one of
	the arguments.  The arguments may be strings or bare words that match a SIP
	method name.  Method names are case-insensitive.

	(hasheader s) - Returns a match if the string argument is the name of a
	field in the SIP message's headers.  The match is case insensitive and will
	will match across SIP long/short form headers (such as "To/t" or "Call-ID/i").

	(header s re) - Returns a match if any header with the same name as the first
	argument matches the regular expression given as the second argument.  If the
	same header appears multiple times, such as in the case of Via, each instance
	of the header is compared for a match.  So 'go doc regexp/syntax' for a
	complete description of what regular expression syntax is allowed.  As with
	hasheader, it will match across SIP long/short form headers.

	(to re) - Returns a match if the To/t header matches the given regular
	expression.  This is a convenience form of (header "to" re).

	(from re) - Returns a match if the From/f header matches the given regular
	expression.  This is a convenience form of (header "from" re).

	(body re) - Returns a match if the SIP message's body contains text matching the
	regular expression argument.

	(message re) - Returns a match if the SIP message's contains text matching the
	regular expression argument anywhere in any header or the entire body.

	(all f ...) - Returns a match if each and every one of the given functions
	evaluate to a match.  The arguments must be a list of 1 or more other
	functions.

	(any f ...) - Returns a match if at least one of the given functions
	evaluate to a match.  The arguments must be a list of 1 or more other
	functions.

	(not f) - Returns a match if the given function would not have matched.

Examples

	""

Any empty rule always matches every SIP Message.

	request

Matches any request, but no responses.

	(status 100 180 183 200)

Matches any response with the numeric status code 100, 180, 183, or 200; this
would be useful for capturing non-error responses to INVITE requests.

	(hasheader "diversion")

Matches any request or response which contains a "Diversion" header.

	(not (status 200))

Matches any request and any response that's not a 200, since status() will return
false for any request.

	(all (method invite) (status 200))

Matches any accepted Invite messages, but not, for example, accepted Publishes
or rejected or provisionally accepted Invites.

	(all (to "alice@provider.com")
	     (method invite bye)
		 (any request (status 200)))

Match any requests or accepted responses to Invites or terminations for calls
destined to alice@provider.com.  This is the sort of rule that could be used to
log VoIP usage for Alice for billing or support purposes.

	(all request
	     (method invite publish)
		 (not (body "don't capture"))
		 (any (header "contact" "alice@.*provider.com")
			  (hasheader "magic")
			  (message "secrets")))

An example of a complex rule.  It captures any request whose method is Invite
or Publish, whose body doesn't contain a particular string and that has at
least one of a special magic header, a secret token in any header or the body,
or is has alice on any host at provider.com as a contact.

*/
package filters
