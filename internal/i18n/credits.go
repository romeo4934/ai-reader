package i18n

// The credits page (art by others used in Lydi).
var creditsStrings = map[string]Dict{
	"en": {
		"CreditsTitle":   "Credits",
		"CreditsIntro":   "The Agora's characters are pixel-art sprites from the Liberated Pixel Cup (LPC), made and shared by these artists under free licenses. Thank you to them!",
		"CreditsOurs":    "Lydi recolors and combines these sprites, and draws the wreaths and the petasos over the LPC heads; these modified sheets are shared under the same licenses, at /static/lpc/.",
		"CreditsBy":      "by",
		"CreditsLicense": "License",
		"CreditsLink":    "Credits",
	},
	"fr": {
		"CreditsTitle":   "Crédits",
		"CreditsIntro":   "Les personnages de l'Agora sont des sprites en pixel art du Liberated Pixel Cup (LPC), créés et partagés sous licence libre par ces artistes. Merci à eux !",
		"CreditsOurs":    "Lydi recolore et assemble ces sprites, et dessine les couronnes et le pétase par-dessus les têtes LPC ; ces planches modifiées sont partagées sous les mêmes licences, dans /static/lpc/.",
		"CreditsBy":      "par",
		"CreditsLicense": "Licence",
		"CreditsLink":    "Crédits",
	},
	"es": {
		"CreditsTitle":   "Créditos",
		"CreditsIntro":   "Los personajes del Ágora son sprites en pixel art del Liberated Pixel Cup (LPC), creados y compartidos con licencias libres por estos artistas. ¡Gracias a ellos!",
		"CreditsOurs":    "Lydi cambia los colores de estos sprites, los combina y dibuja las coronas y el petaso sobre las cabezas LPC; estas hojas modificadas se comparten con las mismas licencias, en /static/lpc/.",
		"CreditsBy":      "por",
		"CreditsLicense": "Licencia",
		"CreditsLink":    "Créditos",
	},
	"pt": {
		"CreditsTitle":   "Créditos",
		"CreditsIntro":   "Os personagens da Ágora são sprites em pixel art do Liberated Pixel Cup (LPC), criados e partilhados com licenças livres por estes artistas. Obrigado a eles!",
		"CreditsOurs":    "O Lydi muda as cores destes sprites, combina-os e desenha as coroas e o pétaso sobre as cabeças LPC; estas folhas modificadas são partilhadas com as mesmas licenças, em /static/lpc/.",
		"CreditsBy":      "por",
		"CreditsLicense": "Licença",
		"CreditsLink":    "Créditos",
	},
	"it": {
		"CreditsTitle":   "Crediti",
		"CreditsIntro":   "I personaggi dell'Agorà sono sprite in pixel art del Liberated Pixel Cup (LPC), creati e condivisi con licenze libere da questi artisti. Grazie a loro!",
		"CreditsOurs":    "Lydi ricolora e combina questi sprite, e disegna le corone e il petaso sopra le teste LPC; questi fogli modificati sono condivisi con le stesse licenze, in /static/lpc/.",
		"CreditsBy":      "di",
		"CreditsLicense": "Licenza",
		"CreditsLink":    "Crediti",
	},
	"de": {
		"CreditsTitle":   "Danksagung",
		"CreditsIntro":   "Die Figuren der Agora sind Pixel-Art-Sprites aus dem Liberated Pixel Cup (LPC), von diesen Künstlerinnen und Künstlern unter freien Lizenzen erstellt und geteilt. Vielen Dank!",
		"CreditsOurs":    "Lydi färbt diese Sprites um, kombiniert sie und zeichnet die Kränze und den Petasos über die LPC-Köpfe; diese veränderten Bögen stehen unter denselben Lizenzen, unter /static/lpc/.",
		"CreditsBy":      "von",
		"CreditsLicense": "Lizenz",
		"CreditsLink":    "Danksagung",
	},
	"nl": {
		"CreditsTitle":   "Credits",
		"CreditsIntro":   "De personages van de Agora zijn pixel-art-sprites uit de Liberated Pixel Cup (LPC), gemaakt en gedeeld onder vrije licenties door deze artiesten. Dank je wel!",
		"CreditsOurs":    "Lydi kleurt deze sprites om, combineert ze en tekent de kransen en de petasos over de LPC-hoofden; deze aangepaste bladen worden onder dezelfde licenties gedeeld, in /static/lpc/.",
		"CreditsBy":      "door",
		"CreditsLicense": "Licentie",
		"CreditsLink":    "Credits",
	},
}

func init() {
	for code, extra := range creditsStrings {
		for k, v := range extra {
			byCode[code][k] = v
		}
	}
}
