package filters

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/matryer/is"
)

func loadSIP(is *is.I, file string) *layers.SIP {
	data, err := ioutil.ReadFile(filepath.Join("testdata", file))
	is.NoErr(err) // loaded sip from file.
	sip := layers.NewSIP()
	err = sip.DecodeFromBytes(data, gopacket.NilDecodeFeedback)
	is.NoErr(err) // can load test SIP packet
	return sip
}

func TestSexpParseFailures(t *testing.T) {
	testCases := map[string]struct {
		src      string
		expected error
	}{
		"empty source":         {``, nil},
		"only whitespace":      {`    `, nil},
		"no function":          {`()`, ErrEmptyExpression},
		"unknown function":     {`doit`, ErrUnknownFunc},
		"only one main":        {`response request`, ErrExtraTokens},
		"empty subexpression":  {`(not ())`, ErrEmptyExpression},
		"mismatched parens":    {`(methods 100 200`, ErrMismatchedParen},
		"hmm":                  {`)`, ErrMismatchedParen},
		"mismatched quote":     {`(to "blah)`, ErrMismatchedQuote},
		"no args":              {`hasheader blah`, ErrExtraTokens},
		"request args":         {`(request blah)`, ErrWrongArgCount},
		"response args":        {`(response blah)`, ErrWrongArgCount},
		"list with int func":   {`(100)`, ErrExpressionType},
		"body - no args":       {`body foo`, ErrExtraTokens},
		"body - many args":     {`(body "foo" "bar")`, ErrWrongArgCount},
		"body - wrong args":    {`(body 100)`, ErrNeedString},
		"body - bad regexp":    {`(body "[")`, ErrBadRegexp},
		"not - no args":        {`not`, ErrWrongArgCount},
		"not - no filter":      {`(not)`, ErrWrongArgCount},
		"not - too many args":  {`(not (any response) request)`, ErrWrongArgCount},
		"not - no function":    {`(not 100)`, ErrExpressionType},
		"not - bad function":   {`(not body foo)`, ErrWrongArgCount},
		"any - no function":    {`(any)`, ErrWrongArgCount},
		"any - bad function":   {`(any doit)`, ErrUnknownFunc}, // falls through to base error
		"all - bad function":   {`(all body)`, ErrWrongArgCount},
		"all - no function":    {`(all)`, ErrWrongArgCount},
		"methods - bad arg":    {`(methods 100)`, ErrMethodsType},
		"methods - bad method": {`(methods foo)`, ErrMethodsType},
		"status - no args":     {`status`, ErrWrongArgCount},
		"status - bad arg":     {`(status foo)`, ErrNeedInt},
		"to - no args":         {`(to)`, ErrWrongArgCount},
		"to - bad arg":         {`(to 18005551212)`, ErrNeedString},
		"from - no args":       {`(from)`, ErrWrongArgCount},
		"from - bad arg":       {`(from bob)`, ErrNeedString},
		"hasheader - arg type": {`(hasheader 100)`, ErrNeedString},
		"hasheader - arg num":  {`(hasheader "foo" "bar")`, ErrWrongArgCount},
		"header - no args":     {`header`, ErrWrongArgCount},
		"header - args few":    {`(header "foo")`, ErrWrongArgCount},
		"header - args many":   {`(header "foo" "bar" "baz")`, ErrWrongArgCount},
		"header - bad regexp":  {`(header "via" "[")`, ErrBadRegexp},
		"header - arg type":    {`(header 100, "alice")`, ErrNeedString},
		"header - arg2 type":   {`(header "via" 100)`, ErrNeedString},
		"message - wrong args": {`(message "[" "that")`, ErrWrongArgCount},
		"message - bad regexp": {`(message "[")`, ErrBadRegexp},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			is := is.New(t)

			_, err := Compile(tc.src)
			t.Log(err, tc.expected)
			is.True(errors.Is(err, tc.expected))
		})
	}
}

func TestCompiler(t *testing.T) {
	is := is.New(t)

	request := loadSIP(is, "invite-request.sip")
	response := loadSIP(is, "invite-response.sip")

	testCases := map[string]struct {
		src    string
		msg    *layers.SIP
		expect bool
	}{
		"empty":          {``, request, true},
		"response pass":  {`response`, response, true},
		"response fail":  {`response`, request, false},
		"request pass":   {`request`, response, false},
		"request fail":   {`request`, request, true},
		"status pass":    {`(status 200)`, response, true},
		"status fail":    {`(status 403)`, response, false},
		"status many":    {`(status 100 180 200)`, response, true},
		"status request": {`(status 200)`, request, false},
		"body pass":      {`(body "(?i:world)")`, request, true},
		"methods pass":   {`(methods invite)`, request, true},
		"methods fail":   {`(methods options)`, request, false},
		"methods many":   {`(methods options invite)`, request, true},
		"methods quoted": {`(methods options "invite")`, request, true},
		"hasheader pass": {`(hasheader "Via")`, request, true},
		"hasheader fail": {`(hasheader "Not-There")`, request, false},
		"to pass":        {`(to "alice")`, request, true},
		"to fail":        {`(to "luigi")`, request, false},
		"from pass":      {`(from "bob")`, request, true},
		"from fail":      {`(from "luigi")`, request, false},
		"header pass":    {`(header "Contact" "bob")`, request, true},
		"header fail":    {`(header "Contact" "alice")`, request, false},
		"message pass":   {`(message "@172.*6{2,3}")`, request, true},
		"message fail":   {`(message "shazam")`, request, false},
		"not pass":       {`(not request)`, response, true},
		"not fail":       {`(not response)`, response, false},
		"any left pass":  {`(any request (hasheader "magic"))`, request, true},
		"any right pass": {`(any request (status 200))`, response, true},
		"any both fail":  {`(any request (hasheader "magic"))`, response, false},
		"all left fail":  {`(all response (status 200))`, request, false},
		"all right fail": {`(all request (hasheader "magic"))`, request, false},
		"all both pass":  {`(all response (status 200))`, response, true},
		"complex": {
			`(all request
				  (methods invite publish)
				  (not (body "don't capture"))
				  (any (to "alice@.*provider.com")
					   (hasheader "magic")
					   (message "secrets")))`,
			request,
			true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			is := is.New(t)
			filter, err := Compile(tc.src)
			is.NoErr(err) // filter compiles
			is.True(filter != nil)
			allow := filter(tc.msg)
			is.Equal(allow, tc.expect) // filter did what it should
		})
	}
}

func BenchmarkCompiler(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Compile(`(all request
				 (methods invite publish)
				 (not (body "don't capture"))
				 (any (header "to" "alice@.*provider.com")
					  (hasheader "magic")
					  (message "secrets")))`)

		if err != nil {
			b.Errorf("unable to compile: %v", err)
		}
	}
}

func BenchmarkComplexFilter(b *testing.B) {
	is := is.New(b)
	filter, err := Compile(`(all request
				 (methods invite publish)
				 (not (body "don't capture"))
				 (any (to "alice@.*provider.com")
					  (hasheader "magic")
					  (message "secrets")))`)
	is.NoErr(err) // filter compiled

	msg := loadSIP(is, "invite-request.sip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(msg)
	}
}

func BenchmarkSimpleFilter(b *testing.B) {
	is := is.New(b)
	filter, err := Compile(`request`)
	is.NoErr(err) // filter compiled

	msg := loadSIP(is, "invite-request.sip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(msg)
	}
}
