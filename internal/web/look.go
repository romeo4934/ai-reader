package web

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
)

// The Agora character's appearance: figure, face (skin, hair, beard) and
// outfit (tunic, cape, headwear). Part of the outfit unlocks with Agora
// rank — readers still at the gates wear the plain colors of the common
// people — which gives one more reason to keep the daily habit. Characters
// are LPC sprites (static/lpc.js): each option names the LPC item or
// palette that draws it, and the swatches show that palette's main tone.

// lookColor is a color choice: the LPC palette and its swatch.
type lookColor struct{ Name, Hex string }

var (
	lookSkins = []lookColor{
		{"light", "#f9d5ba"}, {"amber", "#fdd082"}, {"olive", "#d38b59"},
		{"taupe", "#ba8454"}, {"bronze", "#ae6b3f"}, {"brown", "#9c663e"},
	}
	lookHairColors = []lookColor{
		{"black", "#31313e"}, {"dark_brown", "#5f1f04"}, {"chestnut", "#b6550e"}, {"blonde", "#fccf56"},
		{"platinum", "#eddf95"}, {"gray", "#aaaaaa"}, {"orange", "#e55600"},
	}
	lookBodies     = []string{"male", "female"}
	lookHairStyles = []string{"plain", "long", "bun", "curly", "", "ponytail"} // short, long, bun, curly, bald, ponytail
	lookBeards     = []string{"", "short", "long"}
)

// lookOption is something to wear, from a given rank tier on (-1: everyone,
// 0: citizen, 1: orator, 2: philosopher, 3: sage); Sprite is the LPC cloth
// color (tunics, capes) or headwear it is drawn with.
type lookOption struct {
	Key, Sprite string
	Tier        int
}

var lookTunics = []lookOption{
	{"grey", "gray", -1}, {"beige", "tan", -1}, {"brown", "brown", -1},
	{"terracotta", "orange", 0}, {"olive", "forest", 0}, {"lapis", "blue", 0}, {"navy", "navy", 0}, {"gold", "yellow", 0},
	{"purple", "purple", 0}, {"teal", "teal", 0}, {"rose", "rose", 0}, {"red", "red", 0}, {"maroon", "maroon", 0},
	{"charcoal", "charcoal", 0}, {"white", "white", 0},
}

var lookCapes = []lookOption{
	{"none", "", -1}, {"red", "red", 0}, {"blue", "navy", 0}, {"purple", "purple", 1}, {"gold", "yellow", 3},
}

var lookHeads = []lookOption{
	{"none", "", -1}, {"headband", "headband", -1}, {"petasos", "petasos", 0}, {"olive", "olive", 1},
	{"hood", "hood", 2}, {"laurel", "laurel", 3}, {"crown", "crown", 3},
}

// Look is a character's appearance, stored as
// "bd=1,s=2,h=1,hc=3,b=0,t=lapis,c=red,w=petasos".
type Look struct {
	Body, Skin, Hair, HairColor, Beard int
	Tunic, Cape, Head                  string
}

func (l Look) String() string {
	return fmt.Sprintf("bd=%d,s=%d,h=%d,hc=%d,b=%d,t=%s,c=%s,w=%s", l.Body, l.Skin, l.Hair, l.HairColor, l.Beard, l.Tunic, l.Cape, l.Head)
}

// defaultLook gives everyone a different starting face (from their id), in
// a plain tunic.
func defaultLook(userID int64) Look {
	h := fnv.New32a()
	fmt.Fprint(h, userID)
	n := int(h.Sum32())
	l := Look{
		Skin: n % len(lookSkins), Hair: (n / 7) % len(lookHairStyles), HairColor: (n / 49) % len(lookHairColors),
		Beard: (n / 343) % len(lookBeards), Tunic: lookTunics[(n/2401)%3].Key, Cape: "none", Head: "none",
		Body: (n / 7203) % len(lookBodies),
	}
	if l.Body == 1 {
		l.Beard = 0
	}
	return l
}

func optionTier(opts []lookOption, key string) (int, bool) {
	for _, o := range opts {
		if o.Key == key {
			return o.Tier, true
		}
	}
	return 0, false
}

func optionSprite(opts []lookOption, key string) string {
	for _, o := range opts {
		if o.Key == key {
			return o.Sprite
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
	hasBody := false
	for _, part := range strings.Split(stored, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(v)
		switch k {
		case "bd":
			if n >= 0 && n < len(lookBodies) {
				l.Body, hasBody = n, true
			}
		case "s":
			if n >= 0 && n < len(lookSkins) {
				l.Skin = n
			}
		case "h":
			if n >= 0 && n < len(lookHairStyles) {
				l.Hair = n
			}
		case "hc":
			if n >= 0 && n < len(lookHairColors) {
				l.HairColor = n
			}
		case "b":
			if n >= 0 && n < len(lookBeards) {
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
	// Looks saved before the figure was a choice: a beard means a man.
	if !hasBody && l.Beard > 0 {
		l.Body = 0
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

// lookJSON is the LPC character a look draws (the spec static/lpc.js
// builds sprites from).
type lookJSON struct {
	Body      string `json:"body"`
	Skin      string `json:"skin"`
	Hair      string `json:"hair"`
	HairColor string `json:"hairColor"`
	Beard     string `json:"beard"`
	Tunic     string `json:"tunic"`
	Cape      string `json:"cape"`
	Head      string `json:"head"`
}

func (l Look) json() lookJSON {
	return lookJSON{
		Body: lookBodies[l.Body], Skin: lookSkins[l.Skin].Name, Hair: lookHairStyles[l.Hair],
		HairColor: lookHairColors[l.HairColor].Name, Beard: lookBeards[l.Beard],
		Tunic: optionSprite(lookTunics, l.Tunic), Cape: optionSprite(lookCapes, l.Cape), Head: optionSprite(lookHeads, l.Head),
	}
}

func (s *Server) handleAgoraLook(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	days, err := s.store.CompletedDayCount(user.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	submitted := fmt.Sprintf("bd=%s,s=%s,h=%s,hc=%s,b=%s,t=%s,c=%s,w=%s",
		r.FormValue("body"), r.FormValue("skin"), r.FormValue("hair"), r.FormValue("haircolor"), r.FormValue("beard"),
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
	Look                                          Look
	Bodies, Skins, HairColors, HairStyles, Beards []lookChoice
	Tunics, Capes, Heads                          []lookChoice
	Palettes                                      map[string]any
}

func newLookEditor(T map[string]string, look Look, tier int) lookEditor {
	rankName := func(t int) string {
		if t < 0 {
			return ""
		}
		return T[agoraRanks[t].Key]
	}
	e := lookEditor{Look: look}
	for i := range lookBodies {
		e.Bodies = append(e.Bodies, lookChoice{Value: strconv.Itoa(i)})
	}
	for i, c := range lookSkins {
		e.Skins = append(e.Skins, lookChoice{Value: strconv.Itoa(i), Hex: c.Hex})
	}
	for i, c := range lookHairColors {
		e.HairColors = append(e.HairColors, lookChoice{Value: strconv.Itoa(i), Hex: c.Hex})
	}
	for i := range lookHairStyles {
		e.HairStyles = append(e.HairStyles, lookChoice{Value: strconv.Itoa(i), Label: T["LookHair"+strconv.Itoa(i)]})
	}
	for i := range lookBeards {
		e.Beards = append(e.Beards, lookChoice{Value: strconv.Itoa(i), Label: T["LookBeard"+strconv.Itoa(i)]})
	}
	choices := func(opts []lookOption, labelPrefix string) []lookChoice {
		var out []lookChoice
		for _, o := range opts {
			c := lookChoice{Value: o.Key, Locked: o.Tier > tier, Need: rankName(o.Tier)}
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
	// What static/agora.js needs to turn the form's values into a sprite.
	names := func(cs []lookColor) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Name)
		}
		return out
	}
	sprites := func(opts []lookOption) map[string]string {
		out := map[string]string{}
		for _, o := range opts {
			out[o.Key] = o.Sprite
		}
		return out
	}
	e.Palettes = map[string]any{
		"bodies": lookBodies, "skins": names(lookSkins), "hairColors": names(lookHairColors),
		"hairStyles": lookHairStyles, "beards": lookBeards,
		"tunics": sprites(lookTunics), "capes": sprites(lookCapes), "heads": sprites(lookHeads),
	}
	return e
}
