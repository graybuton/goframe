package goframe

import "testing"

func TestRouteContextQueryParsesRawQuery(t *testing.T) {
	query := RouteContext{
		RawQuery: "status=open&status=review&q=auth+flow&flag&empty=",
	}.Query()

	if got := query.Get("q"); got != "auth flow" {
		t.Fatalf("q = %q, want auth flow", got)
	}
	if !query.Has("flag") {
		t.Fatal("flag should be present")
	}
	if got := query.Get("flag"); got != "" {
		t.Fatalf("flag = %q, want empty", got)
	}
	if got := query.Get("empty"); got != "" {
		t.Fatalf("empty = %q, want empty", got)
	}
	status := query["status"]
	if len(status) != 2 || status[0] != "open" || status[1] != "review" {
		t.Fatalf("status = %#v, want open/review", status)
	}
}

func TestQueryValuesHandlesEmptyAndMissingKeys(t *testing.T) {
	query := ParseQuery("")
	if query.Has("missing") {
		t.Fatal("empty query should not have missing")
	}
	if got := query.Get("missing"); got != "" {
		t.Fatalf("missing = %q, want empty", got)
	}
	if got := (QueryValues(nil)).Get("missing"); got != "" {
		t.Fatalf("nil query Get = %q, want empty", got)
	}
	if (QueryValues(nil)).Has("missing") {
		t.Fatal("nil query Has should be false")
	}
}

func TestParseQueryDecodesCommonEscapes(t *testing.T) {
	query := ParseQuery("q=auth%2Flogin&space=hello+world&bad=%zz")

	if got := query.Get("q"); got != "auth/login" {
		t.Fatalf("q = %q, want auth/login", got)
	}
	if got := query.Get("space"); got != "hello world" {
		t.Fatalf("space = %q, want hello world", got)
	}
	if got := query.Get("bad"); got != "%zz" {
		t.Fatalf("bad = %q, want literal malformed percent", got)
	}
}

func TestQueryValuesEncodeIsDeterministic(t *testing.T) {
	values := QueryValues{
		"status": {"open", "review"},
		"q":      {"auth flow"},
		"flag":   nil,
	}

	if got := values.Encode(); got != "flag&q=auth+flow&status=open&status=review" {
		t.Fatalf("Encode = %q", got)
	}
}

func TestWithQueryReplacesExistingQuery(t *testing.T) {
	got := WithQuery("/issues?old=true", QueryValues{
		"q":      {"auth/login"},
		"status": {"open"},
	})
	if got != "/issues?q=auth%2Flogin&status=open" {
		t.Fatalf("WithQuery = %q", got)
	}
	if href := HashHref(got); href != "#/issues?q=auth%2Flogin&status=open" {
		t.Fatalf("HashHref(WithQuery) = %q", href)
	}
}

func TestWithQueryClearsQueryForEmptyValues(t *testing.T) {
	if got := WithQuery("#/issues?status=open", nil); got != "/issues" {
		t.Fatalf("WithQuery empty = %q, want /issues", got)
	}
}
