package i18n

// Account pages (login, signup, email verification, password reset), the
// emails they send, and the free-plan quota messages. Strings with %s / %d
// are fmt verbs filled in by the caller.
var account = map[string]Dict{
	"en": {
		"AuthLoginTitle": "Log in", "AuthLogin": "Email", "AuthPassword": "Password",
		"AuthLoginBtn": "Log in", "AuthNoAccount": "No account yet?", "AuthSignupLink": "Sign up for free",
		"AuthForgotLink":  "Forgot your password?",
		"AuthSignupTitle": "Create your account", "AuthEmail": "Email", "AuthPasswordHint": "At least 8 characters.",
		"AuthSignupBtn": "Create my account", "AuthHaveAccount": "Already have an account?", "AuthLoginLink": "Log in",
		"AuthFreeNote":    "Free: %d translations per month. No card needed.",
		"AuthErrBadLogin": "Wrong email or password.", "AuthErrBadEmail": "That email address doesn't look valid.",
		"AuthErrShortPassword": "Password too short (8 characters minimum).", "AuthErrEmailTaken": "An account already exists with this email. Log in, or reset your password.",
		"AuthErrTooMany":    "Too many attempts. Wait a few minutes and try again.",
		"AuthErrUnverified": "Confirm your email first: click the link we sent you.", "AuthResend": "Send the link again",
		"AuthCheckTitle": "Check your inbox", "AuthCheckBody": "We sent a link to %s. Click it to activate your account.",
		"AuthCheckSpam":   "Nothing after a few minutes? Check your spam folder.",
		"AuthVerifyBad":   "This link is no longer valid. Log in to get a new one.",
		"AuthForgotTitle": "Reset your password", "AuthForgotBody": "Enter your account's email: we'll send you a link to choose a new password.",
		"AuthForgotBtn": "Send the link", "AuthForgotSent": "If an account exists for %s, a link is on its way. It's valid for one hour.",
		"AuthResetTitle": "Choose a new password", "AuthResetBtn": "Save my password",
		"AuthResetBad":      "This link is no longer valid (expired or already used). Ask for a new one.",
		"MailVerifySubject": "Confirm your ai-reader account",
		"MailVerifyBody":    "Welcome to ai-reader!\n\nClick this link to confirm your email and start reading:\n%s\n\nThe link is valid for 3 days. If you didn't sign up, just ignore this email.",
		"MailResetSubject":  "Reset your ai-reader password",
		"MailResetBody":     "Someone (hopefully you) asked to reset your ai-reader password.\n\nChoose a new one here:\n%s\n\nThe link is valid for one hour. If you didn't ask for this, ignore this email: your password stays the same.",
		"QuotaReached":      "You've used your %d free translations this month. The counter resets on the 1st.",
		"SettingsUsage":     "This month: %d / %d translations", "SettingsUsageUnlimited": "This month: %d translations (unlimited plan)",
	},
	"fr": {
		"AuthLoginTitle": "Connexion", "AuthLogin": "Email", "AuthPassword": "Mot de passe",
		"AuthLoginBtn": "Se connecter", "AuthNoAccount": "Pas encore de compte ?", "AuthSignupLink": "S'inscrire gratuitement",
		"AuthForgotLink":  "Mot de passe oublié ?",
		"AuthSignupTitle": "Créer ton compte", "AuthEmail": "Email", "AuthPasswordHint": "8 caractères minimum.",
		"AuthSignupBtn": "Créer mon compte", "AuthHaveAccount": "Déjà un compte ?", "AuthLoginLink": "Se connecter",
		"AuthFreeNote":    "Gratuit : %d traductions par mois. Pas de carte bancaire.",
		"AuthErrBadLogin": "Email ou mot de passe incorrect.", "AuthErrBadEmail": "Cette adresse email n'a pas l'air valide.",
		"AuthErrShortPassword": "Mot de passe trop court (8 caractères minimum).", "AuthErrEmailTaken": "Un compte existe déjà avec cet email. Connecte-toi, ou réinitialise ton mot de passe.",
		"AuthErrTooMany":    "Trop de tentatives. Attends quelques minutes et réessaie.",
		"AuthErrUnverified": "Confirme d'abord ton email : clique sur le lien qu'on t'a envoyé.", "AuthResend": "Renvoyer le lien",
		"AuthCheckTitle": "Regarde tes emails", "AuthCheckBody": "On t'a envoyé un lien à %s. Clique dessus pour activer ton compte.",
		"AuthCheckSpam":   "Rien après quelques minutes ? Regarde dans tes spams.",
		"AuthVerifyBad":   "Ce lien n'est plus valide. Connecte-toi pour en recevoir un nouveau.",
		"AuthForgotTitle": "Réinitialiser ton mot de passe", "AuthForgotBody": "Entre l'email de ton compte : on t'envoie un lien pour choisir un nouveau mot de passe.",
		"AuthForgotBtn": "Envoyer le lien", "AuthForgotSent": "Si un compte existe pour %s, un lien est en route. Il est valable une heure.",
		"AuthResetTitle": "Choisis un nouveau mot de passe", "AuthResetBtn": "Enregistrer mon mot de passe",
		"AuthResetBad":      "Ce lien n'est plus valide (expiré ou déjà utilisé). Demandes-en un nouveau.",
		"MailVerifySubject": "Confirme ton compte ai-reader",
		"MailVerifyBody":    "Bienvenue sur ai-reader !\n\nClique sur ce lien pour confirmer ton email et commencer à lire :\n%s\n\nLe lien est valable 3 jours. Si tu ne t'es pas inscrit, ignore simplement cet email.",
		"MailResetSubject":  "Réinitialise ton mot de passe ai-reader",
		"MailResetBody":     "Quelqu'un (toi, normalement) a demandé à réinitialiser ton mot de passe ai-reader.\n\nChoisis-en un nouveau ici :\n%s\n\nLe lien est valable une heure. Si ce n'est pas toi, ignore cet email : ton mot de passe ne change pas.",
		"QuotaReached":      "Tu as utilisé tes %d traductions gratuites ce mois-ci. Le compteur repart à zéro le 1er.",
		"SettingsUsage":     "Ce mois-ci : %d / %d traductions", "SettingsUsageUnlimited": "Ce mois-ci : %d traductions (formule illimitée)",
	},
	"es": {
		"AuthLoginTitle": "Iniciar sesión", "AuthLogin": "Email", "AuthPassword": "Contraseña",
		"AuthLoginBtn": "Entrar", "AuthNoAccount": "¿Aún no tienes cuenta?", "AuthSignupLink": "Regístrate gratis",
		"AuthForgotLink":  "¿Olvidaste tu contraseña?",
		"AuthSignupTitle": "Crea tu cuenta", "AuthEmail": "Email", "AuthPasswordHint": "Mínimo 8 caracteres.",
		"AuthSignupBtn": "Crear mi cuenta", "AuthHaveAccount": "¿Ya tienes cuenta?", "AuthLoginLink": "Inicia sesión",
		"AuthFreeNote":    "Gratis: %d traducciones al mes. Sin tarjeta.",
		"AuthErrBadLogin": "Email o contraseña incorrectos.", "AuthErrBadEmail": "Esa dirección de email no parece válida.",
		"AuthErrShortPassword": "Contraseña demasiado corta (mínimo 8 caracteres).", "AuthErrEmailTaken": "Ya existe una cuenta con este email. Inicia sesión o restablece tu contraseña.",
		"AuthErrTooMany":    "Demasiados intentos. Espera unos minutos y vuelve a intentarlo.",
		"AuthErrUnverified": "Primero confirma tu email: haz clic en el enlace que te enviamos.", "AuthResend": "Reenviar el enlace",
		"AuthCheckTitle": "Revisa tu correo", "AuthCheckBody": "Te enviamos un enlace a %s. Haz clic en él para activar tu cuenta.",
		"AuthCheckSpam":   "¿Nada después de unos minutos? Revisa la carpeta de spam.",
		"AuthVerifyBad":   "Este enlace ya no es válido. Inicia sesión para recibir uno nuevo.",
		"AuthForgotTitle": "Restablecer tu contraseña", "AuthForgotBody": "Escribe el email de tu cuenta: te enviaremos un enlace para elegir una nueva contraseña.",
		"AuthForgotBtn": "Enviar el enlace", "AuthForgotSent": "Si existe una cuenta para %s, el enlace va en camino. Es válido durante una hora.",
		"AuthResetTitle": "Elige una nueva contraseña", "AuthResetBtn": "Guardar mi contraseña",
		"AuthResetBad":      "Este enlace ya no es válido (caducado o ya usado). Pide uno nuevo.",
		"MailVerifySubject": "Confirma tu cuenta de ai-reader",
		"MailVerifyBody":    "¡Bienvenido a ai-reader!\n\nHaz clic en este enlace para confirmar tu email y empezar a leer:\n%s\n\nEl enlace es válido durante 3 días. Si no te registraste, ignora este email.",
		"MailResetSubject":  "Restablece tu contraseña de ai-reader",
		"MailResetBody":     "Alguien (esperamos que tú) pidió restablecer tu contraseña de ai-reader.\n\nElige una nueva aquí:\n%s\n\nEl enlace es válido durante una hora. Si no lo pediste, ignora este email: tu contraseña no cambia.",
		"QuotaReached":      "Has usado tus %d traducciones gratuitas de este mes. El contador se reinicia el día 1.",
		"SettingsUsage":     "Este mes: %d / %d traducciones", "SettingsUsageUnlimited": "Este mes: %d traducciones (plan ilimitado)",
	},
	"pt": {
		"AuthLoginTitle": "Entrar", "AuthLogin": "Email", "AuthPassword": "Senha",
		"AuthLoginBtn": "Entrar", "AuthNoAccount": "Ainda não tem conta?", "AuthSignupLink": "Cadastre-se grátis",
		"AuthForgotLink":  "Esqueceu a senha?",
		"AuthSignupTitle": "Crie sua conta", "AuthEmail": "Email", "AuthPasswordHint": "Mínimo de 8 caracteres.",
		"AuthSignupBtn": "Criar minha conta", "AuthHaveAccount": "Já tem conta?", "AuthLoginLink": "Entrar",
		"AuthFreeNote":    "Grátis: %d traduções por mês. Sem cartão.",
		"AuthErrBadLogin": "Email ou senha incorretos.", "AuthErrBadEmail": "Esse endereço de email não parece válido.",
		"AuthErrShortPassword": "Senha muito curta (mínimo de 8 caracteres).", "AuthErrEmailTaken": "Já existe uma conta com este email. Entre ou redefina sua senha.",
		"AuthErrTooMany":    "Tentativas demais. Espere alguns minutos e tente de novo.",
		"AuthErrUnverified": "Confirme seu email primeiro: clique no link que enviamos.", "AuthResend": "Reenviar o link",
		"AuthCheckTitle": "Confira seu email", "AuthCheckBody": "Enviamos um link para %s. Clique nele para ativar sua conta.",
		"AuthCheckSpam":   "Nada depois de alguns minutos? Veja a pasta de spam.",
		"AuthVerifyBad":   "Este link não é mais válido. Entre para receber um novo.",
		"AuthForgotTitle": "Redefinir sua senha", "AuthForgotBody": "Digite o email da sua conta: enviaremos um link para escolher uma nova senha.",
		"AuthForgotBtn": "Enviar o link", "AuthForgotSent": "Se existir uma conta para %s, o link está a caminho. Ele vale por uma hora.",
		"AuthResetTitle": "Escolha uma nova senha", "AuthResetBtn": "Salvar minha senha",
		"AuthResetBad":      "Este link não é mais válido (expirado ou já usado). Peça um novo.",
		"MailVerifySubject": "Confirme sua conta ai-reader",
		"MailVerifyBody":    "Boas-vindas ao ai-reader!\n\nClique neste link para confirmar seu email e começar a ler:\n%s\n\nO link vale por 3 dias. Se você não se cadastrou, ignore este email.",
		"MailResetSubject":  "Redefina sua senha do ai-reader",
		"MailResetBody":     "Alguém (esperamos que você) pediu para redefinir sua senha do ai-reader.\n\nEscolha uma nova aqui:\n%s\n\nO link vale por uma hora. Se não foi você, ignore este email: sua senha continua a mesma.",
		"QuotaReached":      "Você usou suas %d traduções grátis deste mês. O contador zera no dia 1º.",
		"SettingsUsage":     "Este mês: %d / %d traduções", "SettingsUsageUnlimited": "Este mês: %d traduções (plano ilimitado)",
	},
	"it": {
		"AuthLoginTitle": "Accedi", "AuthLogin": "Email", "AuthPassword": "Password",
		"AuthLoginBtn": "Accedi", "AuthNoAccount": "Non hai ancora un account?", "AuthSignupLink": "Iscriviti gratis",
		"AuthForgotLink":  "Password dimenticata?",
		"AuthSignupTitle": "Crea il tuo account", "AuthEmail": "Email", "AuthPasswordHint": "Almeno 8 caratteri.",
		"AuthSignupBtn": "Crea il mio account", "AuthHaveAccount": "Hai già un account?", "AuthLoginLink": "Accedi",
		"AuthFreeNote":    "Gratis: %d traduzioni al mese. Nessuna carta richiesta.",
		"AuthErrBadLogin": "Email o password errati.", "AuthErrBadEmail": "Questo indirizzo email non sembra valido.",
		"AuthErrShortPassword": "Password troppo corta (almeno 8 caratteri).", "AuthErrEmailTaken": "Esiste già un account con questa email. Accedi o reimposta la password.",
		"AuthErrTooMany":    "Troppi tentativi. Aspetta qualche minuto e riprova.",
		"AuthErrUnverified": "Prima conferma la tua email: clicca sul link che ti abbiamo inviato.", "AuthResend": "Invia di nuovo il link",
		"AuthCheckTitle": "Controlla la tua email", "AuthCheckBody": "Abbiamo inviato un link a %s. Cliccalo per attivare il tuo account.",
		"AuthCheckSpam":   "Niente dopo qualche minuto? Controlla lo spam.",
		"AuthVerifyBad":   "Questo link non è più valido. Accedi per riceverne uno nuovo.",
		"AuthForgotTitle": "Reimposta la password", "AuthForgotBody": "Inserisci l'email del tuo account: ti invieremo un link per scegliere una nuova password.",
		"AuthForgotBtn": "Invia il link", "AuthForgotSent": "Se esiste un account per %s, il link è in arrivo. È valido per un'ora.",
		"AuthResetTitle": "Scegli una nuova password", "AuthResetBtn": "Salva la password",
		"AuthResetBad":      "Questo link non è più valido (scaduto o già usato). Richiedine uno nuovo.",
		"MailVerifySubject": "Conferma il tuo account ai-reader",
		"MailVerifyBody":    "Benvenuto su ai-reader!\n\nClicca questo link per confermare la tua email e iniziare a leggere:\n%s\n\nIl link è valido per 3 giorni. Se non ti sei iscritto, ignora questa email.",
		"MailResetSubject":  "Reimposta la tua password ai-reader",
		"MailResetBody":     "Qualcuno (speriamo tu) ha chiesto di reimpostare la tua password ai-reader.\n\nScegline una nuova qui:\n%s\n\nIl link è valido per un'ora. Se non sei stato tu, ignora questa email: la password resta la stessa.",
		"QuotaReached":      "Hai usato le tue %d traduzioni gratuite di questo mese. Il contatore si azzera il 1°.",
		"SettingsUsage":     "Questo mese: %d / %d traduzioni", "SettingsUsageUnlimited": "Questo mese: %d traduzioni (piano illimitato)",
	},
	"de": {
		"AuthLoginTitle": "Anmelden", "AuthLogin": "E-Mail", "AuthPassword": "Passwort",
		"AuthLoginBtn": "Anmelden", "AuthNoAccount": "Noch kein Konto?", "AuthSignupLink": "Kostenlos registrieren",
		"AuthForgotLink":  "Passwort vergessen?",
		"AuthSignupTitle": "Konto erstellen", "AuthEmail": "E-Mail", "AuthPasswordHint": "Mindestens 8 Zeichen.",
		"AuthSignupBtn": "Mein Konto erstellen", "AuthHaveAccount": "Schon ein Konto?", "AuthLoginLink": "Anmelden",
		"AuthFreeNote":    "Kostenlos: %d Übersetzungen pro Monat. Keine Karte nötig.",
		"AuthErrBadLogin": "E-Mail oder Passwort falsch.", "AuthErrBadEmail": "Diese E-Mail-Adresse scheint nicht gültig zu sein.",
		"AuthErrShortPassword": "Passwort zu kurz (mindestens 8 Zeichen).", "AuthErrEmailTaken": "Mit dieser E-Mail gibt es schon ein Konto. Melde dich an oder setze dein Passwort zurück.",
		"AuthErrTooMany":    "Zu viele Versuche. Warte ein paar Minuten und versuch es noch einmal.",
		"AuthErrUnverified": "Bestätige zuerst deine E-Mail: Klicke auf den Link, den wir dir geschickt haben.", "AuthResend": "Link erneut senden",
		"AuthCheckTitle": "Schau in dein Postfach", "AuthCheckBody": "Wir haben einen Link an %s geschickt. Klicke darauf, um dein Konto zu aktivieren.",
		"AuthCheckSpam":   "Nach ein paar Minuten noch nichts? Schau im Spam-Ordner nach.",
		"AuthVerifyBad":   "Dieser Link ist nicht mehr gültig. Melde dich an, um einen neuen zu erhalten.",
		"AuthForgotTitle": "Passwort zurücksetzen", "AuthForgotBody": "Gib die E-Mail deines Kontos ein: Wir schicken dir einen Link, um ein neues Passwort zu wählen.",
		"AuthForgotBtn": "Link senden", "AuthForgotSent": "Falls es ein Konto für %s gibt, ist ein Link unterwegs. Er ist eine Stunde gültig.",
		"AuthResetTitle": "Neues Passwort wählen", "AuthResetBtn": "Passwort speichern",
		"AuthResetBad":      "Dieser Link ist nicht mehr gültig (abgelaufen oder schon benutzt). Fordere einen neuen an.",
		"MailVerifySubject": "Bestätige dein ai-reader-Konto",
		"MailVerifyBody":    "Willkommen bei ai-reader!\n\nKlicke auf diesen Link, um deine E-Mail zu bestätigen und mit dem Lesen zu beginnen:\n%s\n\nDer Link ist 3 Tage gültig. Falls du dich nicht registriert hast, ignoriere diese E-Mail einfach.",
		"MailResetSubject":  "Setze dein ai-reader-Passwort zurück",
		"MailResetBody":     "Jemand (hoffentlich du) hat angefragt, dein ai-reader-Passwort zurückzusetzen.\n\nWähle hier ein neues:\n%s\n\nDer Link ist eine Stunde gültig. Falls du das nicht warst, ignoriere diese E-Mail: Dein Passwort bleibt unverändert.",
		"QuotaReached":      "Du hast deine %d kostenlosen Übersetzungen für diesen Monat verbraucht. Der Zähler wird am 1. zurückgesetzt.",
		"SettingsUsage":     "Diesen Monat: %d / %d Übersetzungen", "SettingsUsageUnlimited": "Diesen Monat: %d Übersetzungen (unbegrenzter Tarif)",
	},
	"nl": {
		"AuthLoginTitle": "Inloggen", "AuthLogin": "E-mail", "AuthPassword": "Wachtwoord",
		"AuthLoginBtn": "Inloggen", "AuthNoAccount": "Nog geen account?", "AuthSignupLink": "Gratis aanmelden",
		"AuthForgotLink":  "Wachtwoord vergeten?",
		"AuthSignupTitle": "Maak je account aan", "AuthEmail": "E-mail", "AuthPasswordHint": "Minstens 8 tekens.",
		"AuthSignupBtn": "Mijn account aanmaken", "AuthHaveAccount": "Al een account?", "AuthLoginLink": "Inloggen",
		"AuthFreeNote":    "Gratis: %d vertalingen per maand. Geen creditcard nodig.",
		"AuthErrBadLogin": "Onjuist e-mailadres of wachtwoord.", "AuthErrBadEmail": "Dat e-mailadres lijkt niet geldig.",
		"AuthErrShortPassword": "Wachtwoord te kort (minstens 8 tekens).", "AuthErrEmailTaken": "Er bestaat al een account met dit e-mailadres. Log in of stel je wachtwoord opnieuw in.",
		"AuthErrTooMany":    "Te veel pogingen. Wacht een paar minuten en probeer het opnieuw.",
		"AuthErrUnverified": "Bevestig eerst je e-mailadres: klik op de link die we je stuurden.", "AuthResend": "Link opnieuw sturen",
		"AuthCheckTitle": "Kijk in je inbox", "AuthCheckBody": "We hebben een link gestuurd naar %s. Klik erop om je account te activeren.",
		"AuthCheckSpam":   "Na een paar minuten nog niets? Kijk in je spammap.",
		"AuthVerifyBad":   "Deze link is niet meer geldig. Log in om een nieuwe te krijgen.",
		"AuthForgotTitle": "Wachtwoord opnieuw instellen", "AuthForgotBody": "Vul het e-mailadres van je account in: we sturen je een link om een nieuw wachtwoord te kiezen.",
		"AuthForgotBtn": "Link sturen", "AuthForgotSent": "Als er een account bestaat voor %s, is er een link onderweg. Die is een uur geldig.",
		"AuthResetTitle": "Kies een nieuw wachtwoord", "AuthResetBtn": "Wachtwoord opslaan",
		"AuthResetBad":      "Deze link is niet meer geldig (verlopen of al gebruikt). Vraag een nieuwe aan.",
		"MailVerifySubject": "Bevestig je ai-reader-account",
		"MailVerifyBody":    "Welkom bij ai-reader!\n\nKlik op deze link om je e-mailadres te bevestigen en te beginnen met lezen:\n%s\n\nDe link is 3 dagen geldig. Heb je je niet aangemeld? Negeer deze e-mail dan.",
		"MailResetSubject":  "Stel je ai-reader-wachtwoord opnieuw in",
		"MailResetBody":     "Iemand (hopelijk jij) heeft gevraagd je ai-reader-wachtwoord opnieuw in te stellen.\n\nKies hier een nieuw wachtwoord:\n%s\n\nDe link is een uur geldig. Was jij dit niet? Negeer deze e-mail dan: je wachtwoord blijft hetzelfde.",
		"QuotaReached":      "Je hebt je %d gratis vertalingen van deze maand gebruikt. De teller begint op de 1e opnieuw.",
		"SettingsUsage":     "Deze maand: %d / %d vertalingen", "SettingsUsageUnlimited": "Deze maand: %d vertalingen (onbeperkt abonnement)",
	},
}

// NativeLangFor maps a UI language code to the native_lang value the
// settings dropdown uses, so a new account's translations come out in the
// language its owner was browsing in.
func NativeLangFor(code string) string {
	switch code {
	case "en":
		return "english"
	case "es":
		return "español"
	case "pt":
		return "português do Brasil"
	case "it":
		return "italiano"
	case "de":
		return "deutsch"
	case "nl":
		return "nederlands"
	default:
		return "français"
	}
}

func init() {
	for code, extra := range account {
		for k, v := range extra {
			byCode[code][k] = v
		}
	}
}
