package frequency

import "testing"

func TestOf(t *testing.T) {
	for _, c := range []struct {
		lang, lemma, phrase string
		estimate            int
		want                func(int) bool
	}{
		{"en", "the", "the", 3, func(r int) bool { return r < 10 }},
		{"es-ES", "", "casa", 3, func(r int) bool { return r < 2000 }},
		{"fra", "maison", "maisons", 3, func(r int) bool { return r < 2000 }},
		{"en", "zzzqqx", "zzzqqx", 2, func(r int) bool { return r == NotInList }},
		{"en", "", "for the sake of", 2, func(r int) bool { return r == 1000 }},
		{"ja", "", "猫", 4, func(r int) bool { return r == 7000 }},
	} {
		if got := Of(c.lang, c.lemma, c.phrase, c.estimate); !c.want(got) {
			t.Errorf("Of(%q, %q, %q) = %d", c.lang, c.lemma, c.phrase, got)
		}
	}
}
