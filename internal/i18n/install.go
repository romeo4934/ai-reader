package i18n

// Installing Lydi as an app.
var installStrings = map[string]Dict{
	"en": {
		"InstallSection": "App",
		"InstallTitle":   "Install the Lydi app",
		"InstallLead":    "An icon on your home screen, full screen, no browser bar.",
		"InstallButton":  "Install",
		"InstallIOS1":    "In Safari, tap the Share button",
		"InstallIOS2":    "Scroll down and choose “Add to Home Screen”",
		"InstallIOS3":    "Tap “Add”: Lydi is on your home screen",
		"InstallOther":   "Open your browser's menu (⋮) and choose “Install app” or “Add to Home screen”.",
		"InstallDone":    "✅ You're using the installed app.",
		"InstallLater":   "Later",
	},
	"fr": {
		"InstallSection": "Appli",
		"InstallTitle":   "Installe l'appli Lydi",
		"InstallLead":    "Une icône sur ton écran d'accueil, en plein écran, sans barre de navigateur.",
		"InstallButton":  "Installer",
		"InstallIOS1":    "Dans Safari, touche le bouton Partager",
		"InstallIOS2":    "Descends et choisis « Sur l'écran d'accueil »",
		"InstallIOS3":    "Touche « Ajouter » : Lydi est sur ton écran d'accueil",
		"InstallOther":   "Ouvre le menu de ton navigateur (⋮) et choisis « Installer l'application » ou « Ajouter à l'écran d'accueil ».",
		"InstallDone":    "✅ Tu utilises l'appli installée.",
		"InstallLater":   "Plus tard",
	},
	"es": {
		"InstallSection": "App",
		"InstallTitle":   "Instala la app de Lydi",
		"InstallLead":    "Un icono en tu pantalla de inicio, a pantalla completa, sin barra del navegador.",
		"InstallButton":  "Instalar",
		"InstallIOS1":    "En Safari, toca el botón Compartir",
		"InstallIOS2":    "Baja y elige «Añadir a pantalla de inicio»",
		"InstallIOS3":    "Toca «Añadir»: Lydi está en tu pantalla de inicio",
		"InstallOther":   "Abre el menú del navegador (⋮) y elige «Instalar aplicación» o «Añadir a pantalla de inicio».",
		"InstallDone":    "✅ Estás usando la app instalada.",
		"InstallLater":   "Más tarde",
	},
	"pt": {
		"InstallSection": "App",
		"InstallTitle":   "Instala a app Lydi",
		"InstallLead":    "Um ícone no ecrã principal, em ecrã inteiro, sem barra do navegador.",
		"InstallButton":  "Instalar",
		"InstallIOS1":    "No Safari, toca no botão Partilhar",
		"InstallIOS2":    "Desce e escolhe «Adicionar ao ecrã principal»",
		"InstallIOS3":    "Toca em «Adicionar»: o Lydi está no teu ecrã principal",
		"InstallOther":   "Abre o menu do navegador (⋮) e escolhe «Instalar app» ou «Adicionar ao ecrã principal».",
		"InstallDone":    "✅ Estás a usar a app instalada.",
		"InstallLater":   "Mais tarde",
	},
	"it": {
		"InstallSection": "App",
		"InstallTitle":   "Installa l'app Lydi",
		"InstallLead":    "Un'icona nella schermata Home, a schermo intero, senza barra del browser.",
		"InstallButton":  "Installa",
		"InstallIOS1":    "In Safari, tocca il pulsante Condividi",
		"InstallIOS2":    "Scorri e scegli «Aggiungi alla schermata Home»",
		"InstallIOS3":    "Tocca «Aggiungi»: Lydi è nella tua schermata Home",
		"InstallOther":   "Apri il menu del browser (⋮) e scegli «Installa app» o «Aggiungi a schermata Home».",
		"InstallDone":    "✅ Stai usando l'app installata.",
		"InstallLater":   "Più tardi",
	},
	"de": {
		"InstallSection": "App",
		"InstallTitle":   "Installiere die Lydi-App",
		"InstallLead":    "Ein Symbol auf deinem Startbildschirm, im Vollbild, ohne Browserleiste.",
		"InstallButton":  "Installieren",
		"InstallIOS1":    "Tippe in Safari auf „Teilen“",
		"InstallIOS2":    "Scrolle nach unten und wähle „Zum Home-Bildschirm“",
		"InstallIOS3":    "Tippe auf „Hinzufügen“: Lydi ist auf deinem Home-Bildschirm",
		"InstallOther":   "Öffne das Browsermenü (⋮) und wähle „App installieren“ oder „Zum Startbildschirm hinzufügen“.",
		"InstallDone":    "✅ Du nutzt die installierte App.",
		"InstallLater":   "Später",
	},
	"nl": {
		"InstallSection": "App",
		"InstallTitle":   "Installeer de Lydi-app",
		"InstallLead":    "Een icoon op je beginscherm, schermvullend, zonder browserbalk.",
		"InstallButton":  "Installeren",
		"InstallIOS1":    "Tik in Safari op de deelknop",
		"InstallIOS2":    "Scrol omlaag en kies ‘Zet op beginscherm’",
		"InstallIOS3":    "Tik op ‘Voeg toe’: Lydi staat op je beginscherm",
		"InstallOther":   "Open het menu van je browser (⋮) en kies ‘App installeren’ of ‘Toevoegen aan startscherm’.",
		"InstallDone":    "✅ Je gebruikt de geïnstalleerde app.",
		"InstallLater":   "Later",
	},
}

func init() {
	for code, extra := range installStrings {
		for k, v := range extra {
			byCode[code][k] = v
		}
	}
}
