package web

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
)

// The Agora character's appearance: face (skin, hair, beard) and outfit
// (tunic, cape, headwear). Part of the outfit unlocks with Agora rank —
// readers still at the gates wear the plain colors of the common people —
// which gives one more reason to keep the daily habit. The palettes below
// are mirrored, by index or key, in static/agora.js, which draws them.

var (
	lookSkins      = []string{"#f6d7b8", "#f0c39b", "#d9a57a", "#b67a4f", "#8a5734", "#5e3a22"}
	lookHairColors = []string{"#1f1a17", "#4a2f1c", "#8a5a2b", "#c98d3c", "#e8d39a", "#9a9a9a", "#b5402a"}
	lookHairStyles = 6 // short, long, bun, curly, bald, ponytail
	lookBeards     = 3 // none, short, long
)

// lookOption is something to wear, from a given rank tier on (-1: everyone,
// 0: citizen, 1: orator, 2: philosopher, 3: sage).
type lookOption struct {
	Key, Hex string
	Tier     int
}

var lookTunics = []lookOption{
	{"grey", "#8e8a83", -1}, {"beige", "#cbbd9c", -1}, {"brown", "#8b6b4a", -1},
	{"terracotta", "#c0603a", 0}, {"olive", "#6b7d3a", 0}, {"lapis", "#2f5d9c", 0}, {"gold", "#b8901f", 0},
	{"purple", "#6d4c8e", 0}, {"teal", "#2f7f7a", 0}, {"rose", "#b5546b", 0}, {"white", "#f2ede1", 0},
}

var lookCapes = []lookOption{
	{"none", "", -1}, {"red", "#a83232", 0}, {"blue", "#2f5d9c", 0}, {"purple", "#6d2f8a", 1}, {"gold", "#c9a227", 3},
}

var lookHeads = []lookOption{
	{"none", "", -1}, {"headband", "", -1}, {"petasos", "", 0}, {"olive", "", 1}, {"hood", "", 2}, {"laurel", "", 3},
}

// Look is a character's appearance, stored as "s=2,h=1,hc=3,b=0,t=lapis,c=red,w=petasos".
type Look struct {
	Skin, Hair, HairColor, Beard int
	Tunic, Cape, Head            string
}

func (l Look) String() string {
	return fmt.Sprintf("s=%d,h=%d,hc=%d,b=%d,t=%s,c=%s,w=%s", l.Skin, l.Hair, l.HairColor, l.Beard, l.Tunic, l.Cape, l.Head)
}

// defaultLook gives everyone a different starting face (from their id), in
// a plain tunic.
func defaultLook(userID int64) Look {
	h := fnv.New32a()
	fmt.Fprint(h, userID)
	n := int(h.Sum32())
	return Look{
		Skin: n % len(lookSkins), Hair: (n / 7) % lookHairStyles, HairColor: (n / 49) % len(lookHairColors),
		Beard: (n / 343) % lookBeards, Tunic: lookTunics[(n/2401)%3].Key, Cape: "none", Head: "none",
	}
}

func optionTier(opts []lookOption, key string) (int, bool) {
	for _, o := range opts {
		if o.Key == key {
			return o.Tier, true
		}
	}
	return 0, false
}

func optionHex(opts []lookOption, key string) string {
	for _, o := range opts {
		if o.Key == key {
			return o.Hex
		}
	}
	return ""
}

// parseLook reads a stored look over the user's default, keeping only what
// their rank tier allows (tier -1 when still at the gates): an item worn
// above one's rank falls back to the default.
func parseLook(userID int64, stored string, tier int) Look {
	l := defaultLook(userID)
	def := l
	for _, part := range strings.Split(stored, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(v)
		switch k {
		case "s":
			if n >= 0 && n < len(lookSkins) {
				l.Skin = n
			}
		case "h":
			if n >= 0 && n < lookHairStyles {
				l.Hair = n
			}
		case "hc":
			if n >= 0 && n < len(lookHairColors) {
				l.HairColor = n
			}
		case "b":
			if n >= 0 && n < lookBeards {
				l.Beard = n
			}
		case "t":
			l.Tunic = v
		case "c":
			l.Cape = v
		case "w":
			l.Head = v
		}
	}
	if t, ok := optionTier(lookTunics, l.Tunic); !ok || t > tier {
		l.Tunic = def.Tunic
	}
	if t, ok := optionTier(lookCapes, l.Cape); !ok || t > tier {
		l.Cape = "none"
	}
	if t, ok := optionTier(lookHeads, l.Head); !ok || t > tier {
		l.Head = "none"
	}
	return l
}

// agoraTier is the rank tier of a number of completed challenges: -1 at
// the gates, then citizen (0) to sage (3).
func agoraTier(days int) int {
	t := -1
	for i, r := range agoraRanks {
		if days >= r.Days {
			t = i
		}
	}
	return t
}

// lookJSON is what static/agora.js needs to draw a character.
type lookJSON struct {
	Skin      string `json:"skin"`
	Hair      int    `json:"hair"`
	HairColor string `json:"hairColor"`
	Beard     int    `json:"beard"`
	Tunic     string `json:"tunic"`
	Cape      string `json:"cape"`
	Head      string `json:"head"`
}

func (l Look) json() lookJSON {
	return lookJSON{
		Skin: lookSkins[l.Skin], Hair: l.Hair, HairColor: lookHairColors[l.HairColor], Beard: l.Beard,
		Tunic: optionHex(lookTunics, l.Tunic), Cape: optionHex(lookCapes, l.Cape), Head: l.Head,
	}
}

func (s *Server) handleAgoraLook(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	days, err := s.store.CompletedDayCount(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	submitted := fmt.Sprintf("s=%s,h=%s,hc=%s,b=%s,t=%s,c=%s,w=%s",
		r.FormValue("skin"), r.FormValue("hair"), r.FormValue("haircolor"), r.FormValue("beard"),
		r.FormValue("tunic"), r.FormValue("cape"), r.FormValue("head"))
	look := parseLook(user.ID, submitted, agoraTier(days))
	if err := s.store.SetAvatarLook(user.ID, look.String()); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	// The medallion's symbol is edited in the same panel.
	if sym := r.FormValue("symbol"); r.Form.Has("symbol") {
		if err := s.store.SetAvatar(user.ID, validSymbol(sym), user.AvatarColor); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	http.Redirect(w, r, "/agora", http.StatusSeeOther)
}

// lookChoice is one option in the character editor.
type lookChoice struct {
	Value, Hex, Label string
	Locked            bool
	Need              string // the rank that unlocks it
}

type lookEditor struct {
	Look                                  Look
	Skins, HairColors, HairStyles, Beards []lookChoice
	Tunics, Capes, Heads                  []lookChoice
	Palettes                              map[string]any
}

func newLookEditor(T map[string]string, look Look, tier int) lookEditor {
	rankName := func(t int) string {
		if t < 0 {
			return ""
		}
		return T[agoraRanks[t].Key]
	}
	e := lookEditor{Look: look}
	for i, hex := range lookSkins {
		e.Skins = append(e.Skins, lookChoice{Value: strconv.Itoa(i), Hex: hex})
	}
	for i, hex := range lookHairColors {
		e.HairColors = append(e.HairColors, lookChoice{Value: strconv.Itoa(i), Hex: hex})
	}
	for i := 0; i < lookHairStyles; i++ {
		e.HairStyles = append(e.HairStyles, lookChoice{Value: strconv.Itoa(i), Label: T["LookHair"+strconv.Itoa(i)]})
	}
	for i := 0; i < lookBeards; i++ {
		e.Beards = append(e.Beards, lookChoice{Value: strconv.Itoa(i), Label: T["LookBeard"+strconv.Itoa(i)]})
	}
	choices := func(opts []lookOption, labelPrefix string) []lookChoice {
		var out []lookChoice
		for _, o := range opts {
			c := lookChoice{Value: o.Key, Hex: o.Hex, Locked: o.Tier > tier, Need: rankName(o.Tier)}
			if labelPrefix != "" {
				c.Label = T[labelPrefix+o.Key]
			}
			out = append(out, c)
		}
		return out
	}
	e.Tunics = choices(lookTunics, "")
	e.Capes = choices(lookCapes, "LookCape_")
	e.Heads = choices(lookHeads, "LookHead_")
	tunics, capes := map[string]string{}, map[string]string{}
	for _, o := range lookTunics {
		tunics[o.Key] = o.Hex
	}
	for _, o := range lookCapes {
		capes[o.Key] = o.Hex
	}
	e.Palettes = map[string]any{"skins": lookSkins, "hairColors": lookHairColors, "tunics": tunics, "capes": capes}
	return e
}
