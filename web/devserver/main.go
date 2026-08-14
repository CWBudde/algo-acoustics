// Command devserver serves the browser demo with the headers the demo needs.
//
// The worker instantiates the Go/WASM module with
// WebAssembly.instantiateStreaming, which rejects any response that is not
// served as application/wasm. Generic static servers (including
// "python3 -m http.server" on hosts without a wasm entry in /etc/mime.types)
// fall back to application/octet-stream and break the demo, so the MIME type is
// set explicitly here.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// revalidateCache lets the browser keep the entry but forces a conditional
	// request every time. The demo assets have no content hash in their names,
	// so a stale copy would pin the tab to an old build.
	revalidateCache = "no-cache"
	// staticCache applies to bundled media that only changes when the file is
	// replaced wholesale and whose staleness is harmless for a day.
	staticCache = "public, max-age=86400"
	// scriptType is the MIME type browsers require for classic and module
	// scripts alike.
	scriptType = "text/javascript; charset=utf-8"
)

// contentTypes overrides the platform MIME database for the extensions the demo
// serves, so behaviour does not depend on /etc/mime.types.
var contentTypes = map[string]string{
	".wasm": "application/wasm",
	".js":   scriptType,
	".mjs":  scriptType,
	".css":  "text/css; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".json": "application/json; charset=utf-8",
	".map":  "application/json; charset=utf-8",
	".mp3":  "audio/mpeg",
	".ogg":  "audio/ogg",
	".wav":  "audio/wav",
	".svg":  "image/svg+xml",
}

// cacheControls maps an extension to its Cache-Control policy. Extensions that
// are absent fall back to revalidateCache.
var cacheControls = map[string]string{
	".mp3": staticCache,
	".ogg": staticCache,
	".wav": staticCache,
	".svg": staticCache,
}

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	dir := flag.String("dir", "web", "directory to serve")
	crossOriginIsolated := flag.Bool("coi", false,
		"send COOP/COEP headers so SharedArrayBuffer is available (see docs/web-deployment.md)")

	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("resolve -dir: %v", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		log.Fatalf("serve %s: %v", root, err)
	}

	if !info.IsDir() {
		log.Fatalf("serve %s: not a directory", root)
	}

	log.Printf("serving %s on http://localhost%s (cross-origin isolation: %t)", root, *addr, *crossOriginIsolated)

	server := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(root, *crossOriginIsolated),
		ReadHeaderTimeout: 10 * time.Second,
	}

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

// newHandler serves root as a static site with demo-appropriate headers.
func newHandler(root string, crossOriginIsolated bool) http.Handler {
	files := http.FileServer(http.Dir(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		target := resolve(root, r.URL.Path)
		ext := strings.ToLower(filepath.Ext(target))

		if contentType, ok := contentTypes[ext]; ok {
			w.Header().Set("Content-Type", contentType)
		}

		w.Header().Set("Cache-Control", cacheControlFor(ext))

		if etag, ok := etagFor(target); ok {
			w.Header().Set("ETag", etag)
		}

		if crossOriginIsolated {
			setCrossOriginIsolationHeaders(w.Header())
		}

		files.ServeHTTP(w, r)
	})
}

// cacheControlFor returns the Cache-Control policy for a file extension.
func cacheControlFor(ext string) string {
	if policy, ok := cacheControls[ext]; ok {
		return policy
	}

	return revalidateCache
}

// setCrossOriginIsolationHeaders enables SharedArrayBuffer for the document.
// COEP: require-corp additionally demands that every cross-origin subresource
// opts in via CORS or Cross-Origin-Resource-Policy.
func setCrossOriginIsolationHeaders(header http.Header) {
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	header.Set("Cross-Origin-Embedder-Policy", "require-corp")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
}

// resolve maps a request path to a path under root, never escaping it.
// Directories resolve to their index.html so that "/" is typed and validated
// like the document it actually serves.
func resolve(root, urlPath string) string {
	clean := filepath.Clean(filepath.FromSlash("/" + strings.TrimPrefix(urlPath, "/")))
	target := filepath.Join(root, clean)

	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		return filepath.Join(target, "index.html")
	}

	return target
}

// etagFor derives a weak validator from the file size and modification time so
// that "no-cache" revalidation answers 304 instead of resending the binary.
// It reports false when the path is not a regular file.
func etagFor(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}

	return fmt.Sprintf(`W/"%x-%x"`, info.Size(), info.ModTime().UnixNano()), true
}
