package web

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"strings"
)

// The preview card of a link challenge: what WhatsApp, Telegram or
// iMessage show under the link, drawn in the game's pixel style.

const (
	cardW, cardH = 1200, 630
	// gameKOScore knocks DLingo out, as in the round's health bar.
	gameKOScore = 50
)

// unaccent maps the accented letters of the site's languages to the
// font's.
var unaccent = strings.NewReplacer(
	"À", "A", "Á", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A", "Æ", "AE", "Ç", "C",
	"È", "E", "É", "E", "Ê", "E", "Ë", "E", "Ì", "I", "Í", "I", "Î", "I", "Ï", "I",
	"Ñ", "N", "Ò", "O", "Ó", "O", "Ô", "O", "Õ", "O", "Ö", "O", "Ø", "O", "Œ", "OE",
	"Ù", "U", "Ú", "U", "Û", "U", "Ü", "U", "Ý", "Y", "Ÿ", "Y", "ß", "SS",
)

// pixelFont: 5x7 glyphs, one string of 7 rows each.
var pixelFont = map[rune]string{
	'A': ".###.#...##...#######...##...##...#", 'B': "####.#...##...#####.#...##...#####.",
	'C': ".###.#...##....#....#....#...#.###.", 'D': "####.#...##...##...##...##...#####.",
	'E': "######....#....####.#....#....#####", 'F': "######....#....####.#....#....#....",
	'G': ".###.#...##....#.####...##...#.###.", 'H': "#...##...##...#######...##...##...#",
	'I': "#####..#....#....#....#....#..#####", 'J': "..###...#....#....#....#.#..#..##..",
	'K': "#...##..#.#.#..##...#.#..#..#.#...#", 'L': "#....#....#....#....#....#....#####",
	'M': "#...###.###.#.##.#.##...##...##...#", 'N': "#...##...###..##.#.##..###...##...#",
	'O': ".###.#...##...##...##...##...#.###.", 'P': "####.#...##...#####.#....#....#....",
	'Q': ".###.#...##...##...##.#.##..#..##.#", 'R': "####.#...##...#####.#.#..#..#.#...#",
	'S': ".#####....#.....###.....#....#####.", 'T': "#####..#....#....#....#....#....#..",
	'U': "#...##...##...##...##...##...#.###.", 'V': "#...##...##...##...##...#.#.#...#..",
	'W': "#...##...##...##.#.##.#.##.#.#.#.#.", 'X': "#...##...#.#.#...#...#.#.#...##...#",
	'Y': "#...##...#.#.#...#....#....#....#..", 'Z': "#####....#...#...#...#...#....#####",
	'0': ".###.#...##..###.#.###..##...#.###.", '1': "..#...##....#....#....#....#...###.",
	'2': ".###.#...#....#...#...#...#...#####", '3': "#####...#...#.....#.....##...#.###.",
	'4': "...#...##..#.#.#..#.#####...#....#.", '5': "######....####.....#....##...#.###.",
	'6': "..##..#...#....####.#...##...#.###.", '7': "#####....#...#...#...#....#....#...",
	'8': ".###.#...##...#.###.#...##...#.###.", '9': ".###.#...##...#.####....#...#..##..",
	'!': "..#....#....#....#....#.........#..", '?': ".###.#...#....#...#...#.........#..",
	'.': "..........................##...##..", '/': "....#....#...#...#...#...#....#....",
	':': "......##...##.........##...##......", '-': "...............#####...............",
	'\'': "..#....#...#.......................", ' ': "...................................",
}

// pixelUpper folds a name to what the font can draw: upper case, accents
// dropped, anything else a space.
func pixelUpper(s string) string {
	var b strings.Builder
	for _, r := range unaccent.Replace(strings.ToUpper(s)) {
		if _, ok := pixelFont[r]; !ok {
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func pixelWidth(s string, scale int) int { return len([]rune(s))*6*scale - scale }

// drawPixel writes s at (x, y), each font pixel a scale-sized square, with
// a drop shadow when shadow isn't nil.
func drawPixel(img *image.RGBA, s string, x, y, scale int, c color.Color, shadow color.Color) {
	for _, pass := range []struct {
		off int
		c   color.Color
	}{{scale / 2, shadow}, {0, c}} {
		if pass.c == nil {
			continue
		}
		cx := x
		for _, r := range s {
			g := pixelFont[r]
			for i, p := range g {
				if p != '#' {
					continue
				}
				px, py := cx+(i%5)*scale+pass.off, y+(i/5)*scale+pass.off
				draw.Draw(img, image.Rect(px, py, px+scale, py+scale), image.NewUniform(pass.c), image.Point{}, draw.Src)
			}
			cx += 6 * scale
		}
	}
}

// drawCentered fits s on one line, shrinking the scale if needed.
func drawCentered(img *image.RGBA, s string, y, scale int, c, shadow color.Color) {
	for scale > 2 && pixelWidth(s, scale) > cardW-80 {
		scale--
	}
	drawPixel(img, s, (cardW-pixelWidth(s, scale))/2, y, scale, c, shadow)
}

// gameCard draws the card: the title, DLingo's health bar after the
// challenger's round, their score and the dare.
func gameCard(name string, score *int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	fill := func(r image.Rectangle, c color.Color) {
		draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
	}
	night := color.RGBA{0x1d, 0x12, 0x38, 0xff}
	fill(img.Bounds(), night)
	// Pixel stars and a ground strip, for the arcade look.
	seed := uint32(7)
	for i := 0; i < 60; i++ {
		seed = seed*1664525 + 1013904223
		x, y := int(seed>>8)%cardW, int(seed>>20)%420
		fill(image.Rect(x, y, x+6, y+6), color.RGBA{0x4a, 0x3b, 0x7a, 0xff})
	}
	fill(image.Rect(0, cardH-60, cardW, cardH), color.RGBA{0x3a, 0x2a, 0x1a, 0xff})
	fill(image.Rect(0, cardH-60, cardW, cardH-48), color.RGBA{0x58, 0xa8, 0x3c, 0xff})

	yellow, red, white := color.RGBA{0xff, 0xd2, 0x3f, 0xff}, color.RGBA{0xff, 0x4d, 0x4d, 0xff}, color.RGBA{0xff, 0xff, 0xff, 0xff}
	shadow := color.RGBA{0x00, 0x00, 0x00, 0xff}
	drawCentered(img, "DLINGO DESTRUCTION", 50, 9, yellow, shadow)

	// DLingo's health bar: what the challenger's round left of it.
	hp := 100
	if score != nil {
		hp = max(0, 100-*score*100/gameKOScore)
	}
	bx, by, bw, bh := 200, 175, 800, 56
	fill(image.Rect(bx-8, by-8, bx+bw+8, by+bh+8), white)
	fill(image.Rect(bx, by, bx+bw, by+bh), color.RGBA{0x55, 0x10, 0x10, 0xff})
	fill(image.Rect(bx, by, bx+bw*hp/100, by+bh), color.RGBA{0x7c, 0xd9, 0x3a, 0xff})
	drawPixel(img, "DLINGO", bx+16, by+11, 5, white, shadow)
	if hp == 0 {
		drawPixel(img, "K.O.!", bx+bw-pixelWidth("K.O.!", 5)-16, by+11, 5, yellow, shadow)
	}

	who := pixelUpper(name)
	if rs := []rune(who); len(rs) > 14 {
		who = string(rs[:14])
		if i := strings.LastIndex(who, " "); i >= 6 {
			who = who[:i]
		}
	}
	if score != nil {
		drawCentered(img, fmt.Sprintf("%s: %d/%d", who, *score, gameMaxScore), 290, 10, white, shadow)
		drawCentered(img, "CAN YOU BEAT IT?", 420, 9, red, shadow)
	} else {
		drawCentered(img, who, 290, 10, white, shadow)
		drawCentered(img, "CHALLENGES YOU!", 420, 9, red, shadow)
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// handleGameCard serves a link challenge's preview card.
func (s *Server) handleGameCard(w http.ResponseWriter, r *http.Request) {
	src, err := s.store.GameOpenRound(r.PathValue("token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=600")
	w.Write(gameCard(publicName(src.ChallengerID, src.ChallengerUser, src.ChallengerDisp), src.ChallengerScore))
}
