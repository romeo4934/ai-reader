package catalog

import "testing"

func TestStripBoilerplate(t *testing.T) {
	long := func(s string) string {
		for len(s) < 250 {
			s += " lorem ipsum"
		}
		return s
	}
	in := []Chapter{
		{"1", `"Cover"`},
		{"2", "The Project Gutenberg eBook of X\n\nlicense blah\n\n*** START OF THE PROJECT GUTENBERG EBOOK X ***\n\n" + long("Chapter one text")},
		{"3", long("Chapter two text")},
		{"4", long("Last chapter") + "\n\n*** END OF THE PROJECT GUTENBERG EBOOK X ***\n\nThe full license"},
		{"5", "THE FULL PROJECT GUTENBERG LICENSE " + long("")},
	}
	out := StripBoilerplate(in)
	old := StripBoilerplate([]Chapter{{"1", "The Project Gutenberg EBook\n\n***START OF THE PROJECT GUTENBERG EBOOK Y***\n\n" + long("Old edition") + "\n\n***END OF THE PROJECT GUTENBERG EBOOK Y***\n\nlicense"}})
	if len(old) != 1 || contains(old[0].Content, "Gutenberg") {
		t.Errorf("old-style markers not handled: %+v", old)
	}
	nl := StripBoilerplate([]Chapter{{"1", "Dit Project Gutenberg eBoek\n\n*** START VAN DIT PROJECT GUTENBERG EBOEK Z ***\n\n" + long("Hoofdstuk een") + "\n\n*** EINDE VAN DIT PROJECT GUTENBERG EBOEK Z ***\n\nLicentie"}})
	if len(nl) != 1 || contains(nl[0].Content, "Gutenberg") || contains(nl[0].Content, "Licentie") {
		t.Errorf("Dutch markers not handled: %+v", nl)
	}
	if len(out) != 3 {
		t.Fatalf("got %d chapters, want 3: %+v", len(out), out)
	}
	if out[0].Content[:16] != "Chapter one text" {
		t.Errorf("first chapter starts with %q", out[0].Content[:16])
	}
	for _, ch := range out {
		for _, bad := range []string{"Project Gutenberg", "*** END", "license"} {
			if contains(ch.Content, bad) {
				t.Errorf("chapter %s still contains %q", ch.Title, bad)
			}
		}
	}
}

func TestCatalogIDsUnique(t *testing.T) {
	seen := map[int]bool{}
	for _, b := range Books {
		if seen[b.ID] {
			t.Errorf("duplicate id %d", b.ID)
		}
		seen[b.ID] = true
		if b.Level < Easy || b.Level > Hard || b.Lang == "" || b.Title == "" {
			t.Errorf("bad entry %+v", b)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestUnmarkHeading(t *testing.T) {
	for in, want := range map[string]string{
		"#INDICE#":       "INDICE",
		"#Primavera#":    "Primavera",
		"Plain text":     "Plain text",
		"Issue #3 of #4": "Issue #3 of #4",
		"#":              "#",
	} {
		if got := unmarkHeading(in); got != want {
			t.Errorf("unmarkHeading(%q) = %q, want %q", in, got, want)
		}
	}
}
