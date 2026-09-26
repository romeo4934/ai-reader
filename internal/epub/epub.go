// Package epub extracts plain-text chapters from an .epub file.
//
// It only reads container.xml, the OPF manifest/spine, and strips tags from
// each XHTML document in reading order — no rendering of images or CSS.
// That's enough for a reading-to-learn app where the point is the text.
package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"path"
	"regexp"
	"strings"
)

type Chapter struct {
	Title   string
	Content string // paragraphs separated by "\n\n"
}

type Book struct {
	Title    string
	Author   string
	Language string
	Chapters []Chapter
}

type container struct {
	Rootfiles struct {
		Rootfile []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfile"`
	} `xml:"rootfiles"`
}

type opfPackage struct {
	Metadata struct {
		Title    []string `xml:"title"`
		Creator  []string `xml:"creator"`
		Language []string `xml:"language"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID   string `xml:"id,attr"`
			Href string `xml:"href,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		ItemRefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

// Parse reads a complete .epub file into a Book with one Chapter per spine item.
func Parse(r io.ReaderAt, size int64) (*Book, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("ouverture epub : %w", err)
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}

	containerData, err := readZipFile(files, "META-INF/container.xml")
	if err != nil {
		return nil, err
	}
	var c container
	if err := xml.Unmarshal(containerData, &c); err != nil {
		return nil, fmt.Errorf("container.xml invalide : %w", err)
	}
	if len(c.Rootfiles.Rootfile) == 0 {
		return nil, fmt.Errorf("container.xml sans rootfile")
	}
	opfPath := c.Rootfiles.Rootfile[0].FullPath
	opfDir := path.Dir(opfPath)

	opfData, err := readZipFile(files, opfPath)
	if err != nil {
		return nil, err
	}
	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return nil, fmt.Errorf("opf invalide : %w", err)
	}

	hrefByID := make(map[string]string, len(pkg.Manifest.Items))
	for _, it := range pkg.Manifest.Items {
		hrefByID[it.ID] = it.Href
	}

	book := &Book{
		Title:    firstOr(pkg.Metadata.Title, "Sans titre"),
		Author:   firstOr(pkg.Metadata.Creator, ""),
		Language: firstOr(pkg.Metadata.Language, ""),
	}

	for i, ref := range pkg.Spine.ItemRefs {
		href, ok := hrefByID[ref.IDRef]
		if !ok {
			continue
		}
		full := path.Join(opfDir, href)
		raw, err := readZipFile(files, full)
		if err != nil {
			continue // spine item missing from zip — skip rather than fail the whole book
		}
		text := strings.TrimSpace(htmlToParagraphs(raw))
		if text == "" {
			continue
		}
		book.Chapters = append(book.Chapters, Chapter{
			Title:   fmt.Sprintf("Chapitre %d", i+1),
			Content: text,
		})
	}
	if len(book.Chapters) == 0 {
		return nil, fmt.Errorf("aucun chapitre exploitable dans cet epub")
	}
	return book, nil
}

func readZipFile(files map[string]*zip.File, name string) ([]byte, error) {
	f, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("fichier manquant dans l'epub : %s", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func firstOr(vals []string, fallback string) string {
	if len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
		return strings.TrimSpace(vals[0])
	}
	return fallback
}

var (
	scriptOrStyle  = regexp.MustCompile(`(?is)<(script|style)\b.*?</(script|style)>`)
	blockBoundary  = regexp.MustCompile(`(?i)</(p|div|h1|h2|h3|h4|h5|h6|li|blockquote)>|<br\s*/?>`)
	anyTag         = regexp.MustCompile(`(?s)<[^>]*>`)
	multiBlankLine = regexp.MustCompile(`\n{3,}`)
	multiSpace     = regexp.MustCompile(`[ \t]+`)
)

// htmlToParagraphs strips an XHTML document down to plain text, keeping
// paragraph breaks so the reading view can render one <p> per paragraph and
// the review cards can quote a whole sentence as context.
func htmlToParagraphs(raw []byte) string {
	s := scriptOrStyle.ReplaceAll(raw, nil)
	s = blockBoundary.ReplaceAll(s, []byte("\n\n"))
	s = anyTag.ReplaceAll(s, nil)
	unescaped := html.UnescapeString(string(s))
	unescaped = multiSpace.ReplaceAllString(unescaped, " ")
	lines := strings.Split(unescaped, "\n")
	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteString(strings.TrimSpace(l))
		buf.WriteString("\n")
	}
	out := multiBlankLine.ReplaceAllString(buf.String(), "\n\n")
	return strings.TrimSpace(out)
}
