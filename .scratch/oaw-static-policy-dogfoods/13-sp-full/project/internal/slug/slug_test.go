package slug

import "testing"

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  string
	}{
		"mixed separators": {input: "Policy: no Bridge, no Runtime!", want: "policy-no-bridge-no-runtime"},
		"edge separators":  {input: "  --Fresh Verification--  ", want: "fresh-verification"},
		"unicode":          {input: "发布 检查 2026", want: "发布-检查-2026"},
		"empty":            {input: "", want: ""},
		"only separators":  {input: "... / ...", want: ""},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := Slugify(test.input); got != test.want {
				t.Fatalf("Slugify(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
