package web

import (
	"io/fs"
	"net/http"
)

// Lydi as an installable app (a PWA): the manifest and the service worker
// are served from the site's root, the service worker so that its scope is
// the whole site. Neither is cached, so changes apply on the next visit.
func serveStaticAt(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(staticFS, "static/"+name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	}
}
