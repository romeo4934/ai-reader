package starter

import (
	"strings"
	"testing"
)

func TestListsLoaded(t *testing.T) {
	for _, native := range order {
		for _, lang := range order {
			if lang == native {
				continue
			}
			if n := len(list(lang, native)); n < 150 {
				t.Errorf("%s-%s: %d words", lang, native, n)
			}
		}
	}
}

func TestPickAndDecoys(t *testing.T) {
	words := Pick("en", "fr", 101, 2000, 5)
	if len(words) != 5 {
		t.Fatalf("Pick: %d words", len(words))
	}
	for _, w := range words {
		if w.Rank < 101 || w.Rank > 2000 {
			t.Errorf("%q: rank %d out of band", w.Phrase, w.Rank)
		}
		d := Decoys("en", "fr", w.Rank, w.Translation, false, 3)
		if len(d) != 3 {
			t.Errorf("%q: %d decoys", w.Phrase, len(d))
		}
		for _, x := range d {
			if strings.EqualFold(x, w.Translation) {
				t.Errorf("%q: decoy is the answer", w.Phrase)
			}
		}
	}
}
