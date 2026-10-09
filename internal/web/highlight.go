package web

import (
	"html/template"
	"strings"
	"unicode"
	"unicode/utf8"
)

// highlightPhrase shows the sentence of a saved word's context that holds
// the word (contexts are whole paragraphs), with the word marked. Only a
// whole-word occurrence counts — "the" isn't the end of "mother" — unless
// there is none, as with a phrase cut inside a compound.
func highlightPhrase(context, phrase string) template.HTML {
	esc := template.HTMLEscapeString
	idx := findWord(context, phrase)
	if phrase == "" || idx < 0 {
		return template.HTML(esc(context))
	}
	from, to := sentenceBounds(context, idx, idx+len(phrase))
	return template.HTML(esc(context[from:idx]) + "<mark>" + esc(context[idx:idx+len(phrase)]) + "</mark>" + esc(context[idx+len(phrase):to]))
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

// findWord is the byte offset of the first whole-word occurrence of phrase
// in s — matching case first, then ignoring it — else of any occurrence,
// else -1.
func findWord(s, phrase string) int {
	if phrase == "" {
		return -1
	}
	whole := func(hay, needle string) int {
		for off := 0; ; {
			i := strings.Index(hay[off:], needle)
			if i < 0 {
				return -1
			}
			i += off
			before, _ := utf8.DecodeLastRuneInString(hay[:i])
			after, _ := utf8.DecodeRuneInString(hay[i+len(needle):])
			if (i == 0 || !isWordRune(before)) && (i+len(needle) == len(hay) || !isWordRune(after)) {
				return i
			}
			_, size := utf8.DecodeRuneInString(hay[i:])
			off = i + size
		}
	}
	if i := whole(s, phrase); i >= 0 {
		return i
	}
	// Lowercasing can change byte lengths (rarely): only then skip it.
	if ls, lp := strings.ToLower(s), strings.ToLower(phrase); len(ls) == len(s) && len(lp) == len(phrase) {
		if i := whole(ls, lp); i >= 0 {
			return i
		}
	}
	return strings.Index(s, phrase)
}

// sentenceBounds widens [from, to) to the sentence around it: back to just
// after the previous sentence end, forward through the next one (with any
// closing quote). A period after a short capitalized word ("Mr.", "Dr.",
// "Sra.") doesn't end a sentence.
func sentenceBounds(s string, from, to int) (int, int) {
	endsSentence := func(i int) bool { // s[i] is a sentence-ending mark
		if s[i] != '.' {
			return true
		}
		j := i
		for j > 0 {
			r, size := utf8.DecodeLastRuneInString(s[:j])
			if !unicode.IsLetter(r) {
				break
			}
			j -= size
		}
		word := s[j:i]
		first, _ := utf8.DecodeRuneInString(word)
		return !(word != "" && utf8.RuneCountInString(word) <= 3 && unicode.IsUpper(first))
	}
	isEnd := func(r rune) bool { return r == '.' || r == '!' || r == '?' || r == '…' || r == '\n' }
	start := 0
	for i := from - 1; i >= 0; i-- {
		r, _ := utf8.DecodeRuneInString(s[i:])
		if !utf8.RuneStart(s[i]) || !isEnd(r) {
			continue
		}
		// a mark followed by a space (after any closing quotes) ends the
		// previous sentence
		j := i + utf8.RuneLen(r)
		for j < from {
			q, size := utf8.DecodeRuneInString(s[j:])
			if !strings.ContainsRune(`"'”’»)`, q) {
				break
			}
			j += size
		}
		if j < from && unicode.IsSpace(rune(s[j])) && (r == '\n' || endsSentence(i)) {
			start = j
			break
		}
	}
	for start < from && unicode.IsSpace(rune(s[start])) {
		start++
	}
	end := len(s)
	for i := to; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if !isEnd(r) || (r == '.' && !endsSentence(i-size)) {
			continue
		}
		for i < len(s) && strings.ContainsRune(`.!?…"'”’»)`, rune(s[i])) {
			i++
		}
		// multi-byte closing quotes
		for i < len(s) {
			q, size := utf8.DecodeRuneInString(s[i:])
			if !strings.ContainsRune(`”’»`, q) {
				break
			}
			i += size
		}
		if i == len(s) || unicode.IsSpace(rune(s[i])) {
			end = i
			break
		}
	}
	return start, end
}
