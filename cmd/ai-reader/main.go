// Command ai-reader serves the reading-to-learn app: upload an epub, tap a
// phrase for a contextual translation, review saved vocab in a Leitner deck.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	_ "time/tzdata" // the tz cookie names IANA zones; don't depend on the host having them

	"github.com/romeo4934/ai-reader/internal/ai"
	"github.com/romeo4934/ai-reader/internal/mail"
	"github.com/romeo4934/ai-reader/internal/store"
	"github.com/romeo4934/ai-reader/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	addr := getenv("AI_READER_ADDR", "127.0.0.1:8091")
	dbPath := getenv("AI_READER_DB", "./ai-reader.sqlite")
	secret := os.Getenv("AI_READER_SECRET")
	if len(secret) < 32 {
		return fmt.Errorf("AI_READER_SECRET manquante ou trop courte (32+ caractères requis) : générer avec `openssl rand -hex 32`")
	}
	freeQuota, err := strconv.Atoi(getenv("AI_READER_FREE_QUOTA", "300"))
	if err != nil || freeQuota < 0 {
		return fmt.Errorf("AI_READER_FREE_QUOTA invalide : %q", os.Getenv("AI_READER_FREE_QUOTA"))
	}
	mailer := mail.New(os.Getenv("CLOUDFLARE_ACCOUNT_ID"), os.Getenv("CLOUDFLARE_API_TOKEN"),
		getenv("AI_READER_MAIL_FROM", "noreply@getlydi.com"), "Lydi", log)
	if !mailer.Enabled() {
		log.Warn("CLOUDFLARE_ACCOUNT_ID / CLOUDFLARE_API_TOKEN absents — les emails sont seulement journalisés, pas envoyés")
	}
	cfg := web.Config{
		BaseURL:   strings.TrimRight(getenv("AI_READER_BASE_URL", "https://book.getlydi.com"), "/"),
		FreeQuota: freeQuota,
		// Next to the database, so it lives on the same (backed-up) volume.
		CatalogDir: filepath.Join(filepath.Dir(dbPath), "catalog"),
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("ouverture base : %w", err)
	}
	defer st.Close()

	aiClient := ai.New(os.Getenv("ANTHROPIC_API_KEY"))
	if !aiClient.Enabled() {
		log.Warn("ANTHROPIC_API_KEY absente — la traduction contextuelle est désactivée")
	}

	srv, err := web.New(st, aiClient, log, []byte(secret), mailer, cfg)
	if err != nil {
		return fmt.Errorf("init serveur web : %w", err)
	}

	log.Info("démarrage", "addr", addr, "db", dbPath)
	return http.ListenAndServe(addr, srv.Routes())
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
