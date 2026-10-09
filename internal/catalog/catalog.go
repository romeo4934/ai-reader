// Package catalog is Lydi's built-in library: public-domain books from
// Project Gutenberg that a reader can add in one click instead of
// importing their own epub. The list is curated by hand (popular titles,
// checked one by one on gutenberg.org) and graded by reading difficulty.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Levels, from easiest to hardest.
const (
	Easy   = 1
	Medium = 2
	Hard   = 3
)

type Book struct {
	ID     int // Project Gutenberg ebook number
	Lang   string
	Title  string
	Author string
	Level  int
}

// Source identifies a catalog book in the books table, so the catalog page
// can tell which ones a reader already has.
func (b Book) Source() string { return "gutenberg:" + strconv.Itoa(b.ID) }

var Books = []Book{
	{11, "en", "Alice's Adventures in Wonderland", "Lewis Carroll", Easy},
	{16, "en", "Peter Pan", "J. M. Barrie", Easy},
	{55, "en", "The Wonderful Wizard of Oz", "L. Frank Baum", Easy},
	{46, "en", "A Christmas Carol", "Charles Dickens", Medium},
	{1661, "en", "The Adventures of Sherlock Holmes", "Arthur Conan Doyle", Medium},
	{43, "en", "The Strange Case of Dr. Jekyll and Mr. Hyde", "Robert Louis Stevenson", Medium},
	{35, "en", "The Time Machine", "H. G. Wells", Medium},
	{120, "en", "Treasure Island", "Robert Louis Stevenson", Medium},
	{64317, "en", "The Great Gatsby", "F. Scott Fitzgerald", Medium},
	{84, "en", "Frankenstein", "Mary Shelley", Hard},
	{1342, "en", "Pride and Prejudice", "Jane Austen", Hard},
	{345, "en", "Dracula", "Bram Stoker", Hard},
	{2701, "en", "Moby Dick", "Herman Melville", Hard},

	{32854, "fr", "Arsène Lupin, gentleman-cambrioleur", "Maurice Leblanc", Easy},
	{800, "fr", "Le tour du monde en quatre-vingts jours", "Jules Verne", Easy},
	{4791, "fr", "Voyage au centre de la Terre", "Jules Verne", Medium},
	{62215, "fr", "Le Fantôme de l'Opéra", "Gaston Leroux", Medium},
	{5097, "fr", "Vingt mille lieues sous les mers", "Jules Verne", Medium},
	{13951, "fr", "Les trois mousquetaires", "Alexandre Dumas", Medium},
	{4650, "fr", "Candide, ou l'optimisme", "Voltaire", Medium},
	{17989, "fr", "Le comte de Monte-Cristo, tome I", "Alexandre Dumas", Hard},
	{14155, "fr", "Madame Bovary", "Gustave Flaubert", Hard},
	{17489, "fr", "Les misérables, tome I : Fantine", "Victor Hugo", Hard},

	{36805, "es", "Spanish Tales for Beginners", "E. C. Hills, Louise Reinhardt", Easy},
	{55514, "es", "Cuentos de amor", "Emilia Pardo Bazán", Medium},
	{49836, "es", "Niebla", "Miguel de Unamuno", Medium},
	{60464, "es", "El árbol de la ciencia", "Pío Baroja", Medium},
	{320, "es", "Lazarillo de Tormes", "Anónimo", Hard},
	{2000, "es", "Don Quijote", "Miguel de Cervantes", Hard},

	{77905, "de", "Deutsche Märchen", "Jacob und Wilhelm Grimm", Easy},
	{24571, "de", "Der Struwwelpeter", "Heinrich Hoffmann", Easy},
	{22367, "de", "Die Verwandlung", "Franz Kafka", Medium},
	{35312, "de", "Aus dem Leben eines Taugenichts", "Joseph von Eichendorff", Medium},
	{34811, "de", "Buddenbrooks", "Thomas Mann", Hard},
	{2229, "de", "Faust. Der Tragödie erster Teil", "Johann Wolfgang von Goethe", Hard},

	{52484, "it", "Le avventure di Pinocchio", "Carlo Collodi", Easy},
	{61885, "it", "Ricordi d'infanzia e di scuola", "Edmondo De Amicis", Medium},
	{65391, "it", "Il Conte di Monte-Cristo", "Alexandre Dumas", Medium},
	{45334, "it", "I promessi sposi", "Alessandro Manzoni", Hard},
	{1012, "it", "La Divina Commedia", "Dante Alighieri", Hard},

	{22015, "pt", "As Minas de Salomão", "H. Rider Haggard", Easy},
	{28341, "pt", "Da terra à lua", "Jules Verne", Medium},
	{31347, "pt", "Contos", "Eça de Queirós", Medium},
	{67740, "pt", "Iracema", "José de Alencar", Medium},
	{69187, "pt", "O Cortiço", "Aluísio Azevedo", Medium},
	{55752, "pt", "Dom Casmurro", "Machado de Assis", Medium},
	{54829, "pt", "Memórias Póstumas de Brás Cubas", "Machado de Assis", Hard},
	{40409, "pt", "Os Maias", "Eça de Queirós", Hard},

	{23759, "nl", "Sagen van Koning Arthur en de Ridders van de Tafelronde", "Nelly Montijn-de Fouw", Easy},
	{17337, "nl", "Onder Moeders Vleugels", "Louisa May Alcott", Easy},
	{27309, "nl", "De Reis naar de Maan", "Jules Verne", Medium},
	{26564, "nl", "Ivanhoe", "Walter Scott", Medium},
	{25946, "nl", "Gevoel en verstand", "Jane Austen", Medium},
	{28068, "nl", "Het ivoren aapje", "Herman Teirlinck", Hard},
}

func ByID(id int) (Book, bool) {
	for _, b := range Books {
		if b.ID == id {
			return b, true
		}
	}
	return Book{}, false
}

// Langs lists the catalog's languages in the order they first appear.
func Langs() []string {
	var out []string
	seen := map[string]bool{}
	for _, b := range Books {
		if !seen[b.Lang] {
			seen[b.Lang] = true
			out = append(out, b.Lang)
		}
	}
	return out
}

// Fetcher downloads catalog epubs from Project Gutenberg, keeping a copy in
// dir so each book is fetched from gutenberg.org only once.
type Fetcher struct {
	dir    string
	client *http.Client
}

func NewFetcher(dir string) *Fetcher {
	return &Fetcher{dir: dir, client: &http.Client{Timeout: 90 * time.Second}}
}

const maxEpubBytes = 30 << 20

// Epub returns the book's epub file (the no-images edition: a fraction of
// the size, and Lydi only shows text).
func (f *Fetcher) Epub(ctx context.Context, b Book) ([]byte, error) {
	path := filepath.Join(f.dir, strconv.Itoa(b.ID)+".epub")
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		return data, nil
	}
	// Not every book has every edition: fall back to the ones with images.
	var data []byte
	var err error
	for _, edition := range []string{"epub.noimages", "epub3.images", "epub.images"} {
		data, err = f.download(ctx, fmt.Sprintf("https://www.gutenberg.org/ebooks/%d.%s", b.ID, edition))
		if err == nil || !errors.Is(err, errNotFound) {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(f.dir, 0o750); err == nil {
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o640); err == nil {
			_ = os.Rename(tmp, path)
		}
	}
	return data, nil
}

var errNotFound = errors.New("introuvable")

func (f *Fetcher) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Lydi/1.0 (+https://getlydi.com)")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("téléchargement Gutenberg : %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("téléchargement Gutenberg %s : %w", url, errNotFound)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("téléchargement Gutenberg : statut %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxEpubBytes+1))
	if err != nil {
		return nil, fmt.Errorf("téléchargement Gutenberg : %w", err)
	}
	if len(data) > maxEpubBytes {
		return nil, errors.New("epub Gutenberg trop volumineux")
	}
	return data, nil
}

type Chapter struct{ Title, Content string }

// StripBoilerplate removes what Project Gutenberg wraps around the text:
// the near-empty cover chapter, everything up to the "*** START OF THE
// PROJECT GUTENBERG EBOOK" line, and from "*** END OF" on (license
// included). Chapters left empty are dropped.
func StripBoilerplate(chapters []Chapter) []Chapter {
	started := !hasMarker(chapters, startMarker)
	ended := false
	var out []Chapter
	for _, ch := range chapters {
		if ended {
			break
		}
		paras := strings.Split(ch.Content, "\n\n")
		var keep []string
		for _, p := range paras {
			switch {
			case !started && startMarker.MatchString(p):
				started = true
				keep = keep[:0]
				continue
			case started && endMarker.MatchString(p):
				ended = true
			}
			if ended {
				break
			}
			if started {
				keep = append(keep, p)
			}
		}
		content := strings.TrimSpace(strings.Join(keep, "\n\n"))
		if len(content) < 200 {
			continue // cover, empty title page, or nothing left
		}
		out = append(out, Chapter{Title: ch.Title, Content: content})
	}
	return out
}

// The text sits between two "*** … PROJECT GUTENBERG … ***" lines, worded
// differently across editions ("***START OF THE…", "*** START OF THIS…")
// and languages (Dutch "*** EINDE VAN DIT…"): the first such line opens
// the text, the next one closes it. Some editions also put the license
// before the closing line, and Dutch ones end with a "Colofon" holding it.
var (
	startMarker = regexp.MustCompile(`(?i)^\s*\*\*\*.*gutenberg`)
	endMarker   = regexp.MustCompile(`(?i)^\s*\*\*\*.*gutenberg|^\s*THE FULL PROJECT GUTENBERG|^\s*Colofon\s*$|^\s*End of (the )?Project Gutenberg`)
)

func hasMarker(chapters []Chapter, marker *regexp.Regexp) bool {
	for _, ch := range chapters {
		for _, p := range strings.Split(ch.Content, "\n\n") {
			if marker.MatchString(p) {
				return true
			}
		}
	}
	return false
}
