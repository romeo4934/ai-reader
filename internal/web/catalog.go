package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/romeo4934/ai-reader/internal/catalog"
	"github.com/romeo4934/ai-reader/internal/epub"
)

// The built-in library: pick a public-domain book instead of importing an
// epub. Adding one downloads it from Project Gutenberg (cached), strips
// Gutenberg's header and license, and stores it like an upload.

var genericChapter = regexp.MustCompile(`^Chapitre \d+$`)

type catalogItem struct {
	catalog.Book
	BookID int64 // the reader's copy, 0 if not added yet
}

type catalogLevel struct {
	Name  string
	Items []catalogItem
}

type catalogView struct {
	Tabs   []leaderboardTab
	Levels []catalogLevel
	Error  string
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	langs := catalog.Langs()
	lang := r.URL.Query().Get("lang")
	if !slices.Contains(langs, lang) {
		// The language of the reader's latest book, else the first one
		// that isn't their own.
		lang, _ = s.store.UserBookLang(user.ID)
		if !slices.Contains(langs, lang) {
			lang = langs[0]
			if lang == T["LangCode"] && len(langs) > 1 {
				lang = langs[1]
			}
		}
	}
	have, err := s.store.BookSources(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	v := catalogView{Error: r.URL.Query().Get("err")}
	for _, l := range langs {
		v.Tabs = append(v.Tabs, leaderboardTab{Lang: l, Name: langName(l), Active: l == lang})
	}
	for level, key := range []string{catalog.Easy: "CatalogEasy", catalog.Medium: "CatalogMedium", catalog.Hard: "CatalogHard"} {
		if key == "" {
			continue
		}
		lv := catalogLevel{Name: T[key]}
		for _, b := range catalog.Books {
			if b.Lang == lang && b.Level == level {
				lv.Items = append(lv.Items, catalogItem{Book: b, BookID: have[b.Source()]})
			}
		}
		if len(lv.Items) > 0 {
			v.Levels = append(v.Levels, lv)
		}
	}
	s.render(w, r, "catalog.html", T["CatalogTitle"], v)
}

func (s *Server) handleCatalogAdd(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	T := s.dictFor(r)
	id, _ := strconv.Atoi(r.PathValue("id"))
	b, ok := catalog.ByID(id)
	if !ok {
		s.fail(w, http.StatusNotFound, fmt.Errorf("livre %d absent du catalogue", id))
		return
	}
	have, err := s.store.BookSources(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if bookID, ok := have[b.Source()]; ok {
		http.Redirect(w, r, fmt.Sprintf("/books/%d", bookID), http.StatusSeeOther)
		return
	}
	failed := func(err error) {
		s.log.Error("ajout depuis le catalogue", "gutenberg", b.ID, "err", err)
		http.Redirect(w, r, "/catalog?lang="+b.Lang+"&err="+url.QueryEscape(T["CatalogErr"]), http.StatusSeeOther)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	data, err := s.catalog.Epub(ctx, b)
	if err != nil {
		failed(err)
		return
	}
	parsed, err := epub.Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		failed(err)
		return
	}
	raw := make([]catalog.Chapter, len(parsed.Chapters))
	for i, ch := range parsed.Chapters {
		raw[i] = catalog.Chapter{Title: ch.Title, Content: ch.Content}
	}
	clean := catalog.StripBoilerplate(raw)
	if len(clean) == 0 {
		failed(fmt.Errorf("aucun chapitre après nettoyage"))
		return
	}
	chapters := make([]struct{ Title, Content string }, len(clean))
	for i, ch := range clean {
		// The epub parser numbers chapters by position; the dropped cover
		// would otherwise make the first one "Chapitre 2".
		title := ch.Title
		if genericChapter.MatchString(title) {
			title = fmt.Sprintf("Chapitre %d", i+1)
		}
		chapters[i] = struct{ Title, Content string }{title, ch.Content}
	}
	bookID, err := s.store.InsertBook(user.ID, b.Title, b.Author, b.Lang, b.Source(), chapters)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/books/%d", bookID), http.StatusSeeOther)
}
