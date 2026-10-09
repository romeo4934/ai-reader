package web

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/romeo4934/ai-reader/internal/catalog"
)

// The first run: a reader with no book yet is walked through choosing the
// language they learn, their level, and one of three books from the
// built-in library suited to both — instead of landing on an empty
// library. Each step is a plain link (?lang=…, then &level=…), so the back
// button just works.

type startOption struct {
	Value, Name, Hint string
}

type startView struct {
	Step     int // 1: language, 2: level, 3: books
	Lang     string
	LangName string
	Langs    []startOption
	Levels   []startOption
	Books    []catalog.Book
}

const startBooks = 3

func (s *Server) renderStart(w http.ResponseWriter, r *http.Request) {
	T := s.dictFor(r)
	v := startView{Step: 1}
	langs := catalog.Langs()
	if l := r.URL.Query().Get("lang"); slices.Contains(langs, l) {
		v.Step, v.Lang, v.LangName = 2, l, localLangName(T["LangCode"], l)
		if level, err := strconv.Atoi(r.URL.Query().Get("level")); err == nil && level >= catalog.Easy && level <= catalog.Hard {
			v.Step = 3
			v.Books = suggestBooks(l, level, startBooks)
		}
	}
	for _, l := range langs {
		if l != T["LangCode"] { // not the reader's own language
			v.Langs = append(v.Langs, startOption{Value: l, Name: langName(l)})
		}
	}
	for level, key := range []string{catalog.Easy: "StartLevel1", catalog.Medium: "StartLevel2", catalog.Hard: "StartLevel3"} {
		if key != "" {
			v.Levels = append(v.Levels, startOption{Value: strconv.Itoa(level), Name: T[key], Hint: T[key+"Hint"]})
		}
	}
	s.render(w, r, "start.html", T["StartTitle"], v)
}

// suggestBooks picks n catalog books in lang, at the reader's level first,
// then the nearest levels (easier before harder).
func suggestBooks(lang string, level, n int) []catalog.Book {
	var out []catalog.Book
	for _, l := range []int{level, level - 1, level + 1, level - 2, level + 2} {
		for _, b := range catalog.Books {
			if len(out) == n {
				return out
			}
			if b.Lang == lang && b.Level == l {
				out = append(out, b)
			}
		}
	}
	return out
}

// Language names inside a sentence of the UI ("ton niveau en anglais"), as
// opposed to the tabs, which show each language by its own name.
var localLangNames = map[string]map[string]string{
	"fr": {"en": "anglais", "fr": "français", "es": "espagnol", "de": "allemand", "it": "italien", "pt": "portugais", "nl": "néerlandais"},
	"en": {"en": "English", "fr": "French", "es": "Spanish", "de": "German", "it": "Italian", "pt": "Portuguese", "nl": "Dutch"},
	"es": {"en": "inglés", "fr": "francés", "es": "español", "de": "alemán", "it": "italiano", "pt": "portugués", "nl": "neerlandés"},
	"pt": {"en": "inglês", "fr": "francês", "es": "espanhol", "de": "alemão", "it": "italiano", "pt": "português", "nl": "holandês"},
	"it": {"en": "inglese", "fr": "francese", "es": "spagnolo", "de": "tedesco", "it": "italiano", "pt": "portoghese", "nl": "olandese"},
	"de": {"en": "Englisch", "fr": "Französisch", "es": "Spanisch", "de": "Deutsch", "it": "Italienisch", "pt": "Portugiesisch", "nl": "Niederländisch"},
	"nl": {"en": "Engels", "fr": "Frans", "es": "Spaans", "de": "Duits", "it": "Italiaans", "pt": "Portugees", "nl": "Nederlands"},
}

func localLangName(ui, code string) string {
	if n, ok := localLangNames[ui][code]; ok {
		return n
	}
	return langName(code)
}
