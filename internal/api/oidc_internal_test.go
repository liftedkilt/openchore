package api

import "testing"

func TestSafeReturn(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"/":                     "/",
		"/account?view=x":       "/account?view=x",
		"//evil.example":        "",
		"/\\evil.example":       "",
		"\\\\evil.example":      "",
		"https://evil.example/": "",
		"evil.example":          "",
		"/ok/../path":           "/ok/../path",
		"/a\\b":                 "",
		"/%0d%0aSet-Cookie:x=1": "/%0d%0aSet-Cookie:x=1",
		"/line\nbreak":          "",
		"javascript:alert(1)":   "",
		"/\t/evil.example":      "",
	}
	for in, want := range cases {
		if got := safeReturn(in); got != want {
			t.Errorf("safeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}
