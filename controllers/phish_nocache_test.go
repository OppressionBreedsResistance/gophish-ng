package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoCacheMiddlewareSetsHeader(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		handler http.HandlerFunc
	}{
		{"challenge page", "/static/attachments/1/abc/photos.zip", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte("challenge"))
		}},
		{"error", "/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}},
		{"redirect", "/?keyname=abc", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/static/attachments/1/abc/photos.zip", http.StatusFound)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			noCacheMiddleware(tc.handler).ServeHTTP(rec, req)
			got := rec.Header().Get("Cache-Control")
			if got != "private, no-store, max-age=0" {
				t.Fatalf("Cache-Control = %q, want %q", got, "private, no-store, max-age=0")
			}
		})
	}
}

// The middleware must be wired into the real phishing router, so that every
// response (pages, 404s, hosted attachments, static files) carries the header.
func TestPhishServerResponsesAreNotCacheable(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	for _, path := range []string{"/robots.txt", "/?keyname=doesnotexist", "/static/attachments/1/abc/photos.zip"} {
		resp, err := http.Get(fmt.Sprintf("%s%s", ctx.phishServer.URL, path))
		if err != nil {
			t.Fatalf("error requesting %s: %v", path, err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Cache-Control"); got != "private, no-store, max-age=0" {
			t.Fatalf("%s: Cache-Control = %q, want %q", path, got, "private, no-store, max-age=0")
		}
	}
}
