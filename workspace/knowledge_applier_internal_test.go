package workspace

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple sentence", "Add CSV export to the reports page", "add-csv-export-to-the-reports-page"},
		{"punctuation collapses to one hyphen", "Fix bug!!  in --parser--", "fix-bug-in-parser"},
		{"empty string falls back to note", "", "note"},
		{"no alphanumeric characters falls back to note", "!!!---???", "note"},
		{"long intent is capped at 40 characters", "this is a very long intent that goes on and on and on and on", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := slugify(c.in)
			if c.name == "long intent is capped at 40 characters" {
				if len(got) > 40 {
					t.Errorf("slugify(%q) = %q (%d chars), want at most 40", c.in, got, len(got))
				}
				return
			}
			if got != c.want {
				t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
