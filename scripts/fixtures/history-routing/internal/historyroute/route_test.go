package historyroute

import "testing"

func TestNormalizeTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "empty", target: "", want: "/"},
		{name: "leading slash", target: "users/42", want: "/users/42"},
		{name: "repeated slash", target: "//users///42//", want: "/users/42"},
		{name: "ordinary trailing slash", target: "/users/42/", want: "/users/42"},
		{name: "settings canonical slash", target: "/settings", want: "/settings/"},
		{name: "query preserved", target: "/search?q=a%2Fb", want: "/search?q=a%2Fb"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeTarget(test.target); got != test.want {
				t.Fatalf("NormalizeTarget(%q) = %q, want %q", test.target, got, test.want)
			}
		})
	}
}

func TestMatchTarget(t *testing.T) {
	tests := []struct {
		target         string
		name           string
		param          string
		query          string
		notFound       bool
		malformedPath  bool
		malformedQuery bool
	}{
		{target: "/", name: "home"},
		{target: "/users/42", name: "user", param: "42"},
		{target: "/users/7?tab=activity", name: "user", param: "7", query: "activity"},
		{target: "/users/Ada%20Lovelace", name: "user", param: "Ada Lovelace"},
		{target: "/search?q=goframe", name: "search", query: "goframe"},
		{target: "/settings/", name: "settings"},
		{target: "/does-not-exist", name: "not-found", notFound: true},
		{target: "/users/%ZZ", name: "not-found", notFound: true, malformedPath: true},
		{target: "/search?q=%ZZ", name: "search", malformedQuery: true},
	}
	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			got := MatchTarget(test.target)
			if got.Name != test.name ||
				got.Param != test.param ||
				got.QueryValue != test.query ||
				got.NotFound != test.notFound ||
				got.MalformedPath != test.malformedPath ||
				got.MalformedQuery != test.malformedQuery {
				t.Fatalf("MatchTarget(%q) = %#v", test.target, got)
			}
		})
	}
}

func TestTargetFromLocation(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		pathname string
		search   string
		want     string
		inside   bool
	}{
		{name: "root", base: "/", pathname: "/", want: "/", inside: true},
		{name: "root deep", base: "/", pathname: "/users/42", search: "?tab=activity", want: "/users/42?tab=activity", inside: true},
		{name: "subpath home", base: "/app/", pathname: "/app/", want: "/", inside: true},
		{name: "subpath deep", base: "/app", pathname: "/app/search", search: "q=goframe", want: "/search?q=goframe", inside: true},
		{name: "outside subpath", base: "/app/", pathname: "/users/42", inside: false},
		{name: "prefix sibling", base: "/app/", pathname: "/application/users/42", inside: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, inside, err := TargetFromLocation(test.base, test.pathname, test.search)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want || inside != test.inside {
				t.Fatalf("TargetFromLocation(%q, %q, %q) = %q, %v; want %q, %v", test.base, test.pathname, test.search, got, inside, test.want, test.inside)
			}
		})
	}
}

func TestLocationForTarget(t *testing.T) {
	tests := []struct {
		base   string
		target string
		want   string
	}{
		{base: "/", target: "/", want: "/"},
		{base: "/", target: "/users/42?tab=activity", want: "/users/42?tab=activity"},
		{base: "/app/", target: "/", want: "/app/"},
		{base: "/app", target: "/users/42", want: "/app/users/42"},
		{base: "app", target: "/settings", want: "/app/settings/"},
	}
	for _, test := range tests {
		t.Run(test.base+" "+test.target, func(t *testing.T) {
			got, err := LocationForTarget(test.base, test.target)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("LocationForTarget(%q, %q) = %q, want %q", test.base, test.target, got, test.want)
			}
		})
	}
}

func TestNormalizeBaseRejectsNonPathInput(t *testing.T) {
	for _, base := range []string{"/app/?q=1", "/app/#x", `\app`, "../app", "/app/../"} {
		if _, err := NormalizeBase(base); err == nil {
			t.Fatalf("NormalizeBase(%q) succeeded", base)
		}
	}
}
