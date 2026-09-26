// Command ai-reader serves the reading-to-learn app: upload an epub, tap a
// phrase for a contextual translation, review saved vocab in a Leitner deck.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/romeo4934/ai-reader/internal/ai"
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

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("ouverture base : %w", err)
	}
	defer st.Close()

	aiClient := ai.New(os.Getenv("ANTHROPIC_API_KEY"))
	if !aiClient.Enabled() {
		log.Warn("ANTHROPIC_API_KEY absente — la traduction contextuelle est désactivée")
	}

	srv, err := web.New(st, aiClient, log)
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
