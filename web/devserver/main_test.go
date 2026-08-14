package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTestRoot writes a miniature copy of the demo tree and returns its path.
func newTestRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	files := map[string]string{
		"index.html":                       "<!doctype html>",
		"app.js":                           "export {};",
		"styles.css":                       "body{}",
		"algo_acoustics_demo.wasm":         "\x00asm\x01\x00\x00\x00",
		filepath.Join("audio", "clap.mp3"): "ID3",
	}

	for name, content := range files {
		path := filepath.Join(root, name)

		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}

		err = os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	return root
}

func TestHandlerHeaders(t *testing.T) {
	root := newTestRoot(t)
	handler := newHandler(root, false)

	tests := []struct {
		name         string
		path         string
		contentType  string
		cacheControl string
	}{
		{
			name:         "wasm binary uses the streaming-compilation MIME type",
			path:         "/algo_acoustics_demo.wasm",
			contentType:  "application/wasm",
			cacheControl: revalidateCache,
		},
		{
			name:         "module script",
			path:         "/app.js",
			contentType:  "text/javascript; charset=utf-8",
			cacheControl: revalidateCache,
		},
		{
			name:         "stylesheet",
			path:         "/styles.css",
			contentType:  "text/css; charset=utf-8",
			cacheControl: revalidateCache,
		},
		{
			name:         "document served from the directory root",
			path:         "/",
			contentType:  "text/html; charset=utf-8",
			cacheControl: revalidateCache,
		},
		{
			name:         "bundled audio sample is cacheable",
			path:         "/audio/clap.mp3",
			contentType:  "audio/mpeg",
			cacheControl: staticCache,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}

			if got := recorder.Header().Get("Content-Type"); got != tt.contentType {
				t.Errorf("Content-Type = %q, want %q", got, tt.contentType)
			}

			if got := recorder.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}

			if recorder.Header().Get("ETag") == "" {
				t.Error("ETag is empty, want a validator for conditional requests")
			}
		})
	}
}

func TestHandlerRevalidationReturnsNotModified(t *testing.T) {
	root := newTestRoot(t)
	handler := newHandler(root, false)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/algo_acoustics_demo.wasm", nil))

	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response carried no ETag")
	}

	request := httptest.NewRequest(http.MethodGet, "/algo_acoustics_demo.wasm", nil)
	request.Header.Set("If-None-Match", etag)

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)

	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", second.Code, http.StatusNotModified)
	}

	if second.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", second.Body.Len())
	}
}

func TestHandlerCrossOriginIsolation(t *testing.T) {
	root := newTestRoot(t)

	tests := []struct {
		name     string
		isolated bool
		wantCOOP string
		wantCOEP string
	}{
		{name: "disabled by default", isolated: false, wantCOOP: "", wantCOEP: ""},
		{
			name:     "enabled with -coi",
			isolated: true,
			wantCOOP: "same-origin",
			wantCOEP: "require-corp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			newHandler(root, tt.isolated).ServeHTTP(
				recorder,
				httptest.NewRequest(http.MethodGet, "/index.html", nil),
			)

			if got := recorder.Header().Get("Cross-Origin-Opener-Policy"); got != tt.wantCOOP {
				t.Errorf("COOP = %q, want %q", got, tt.wantCOOP)
			}

			if got := recorder.Header().Get("Cross-Origin-Embedder-Policy"); got != tt.wantCOEP {
				t.Errorf("COEP = %q, want %q", got, tt.wantCOEP)
			}
		})
	}
}

func TestHandlerRejectsNonReadMethods(t *testing.T) {
	root := newTestRoot(t)
	recorder := httptest.NewRecorder()

	newHandler(root, false).ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/index.html", nil),
	)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func TestEtagForMissingFile(t *testing.T) {
	root := newTestRoot(t)

	if _, ok := etagFor(filepath.Join(root, "does-not-exist.wasm")); ok {
		t.Error("etagFor returned a validator for a missing file")
	}

	if _, ok := etagFor(filepath.Join(root, "audio")); ok {
		t.Error("etagFor returned a validator for a directory")
	}
}

func TestResolve(t *testing.T) {
	root := newTestRoot(t)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "file", path: "/app.js", want: filepath.Join(root, "app.js")},
		{name: "nested file", path: "/audio/clap.mp3", want: filepath.Join(root, "audio", "clap.mp3")},
		{name: "directory root", path: "/", want: filepath.Join(root, "index.html")},
		{name: "nested directory", path: "/audio", want: filepath.Join(root, "audio", "index.html")},
		{name: "traversal stays inside root", path: "/../../etc/passwd", want: filepath.Join(root, "etc", "passwd")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(root, tt.path); got != tt.want {
				t.Errorf("resolve(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
