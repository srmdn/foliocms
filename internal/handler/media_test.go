package handler_test

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/srmdn/foliocms/internal/media"
)

func uploadMedia(t *testing.T, r *chi.Mux, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/media", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newMediaRouter(t *testing.T) *chi.Mux {
	t.Helper()
	h := newTestHandler(t, t.TempDir())
	h.SetMediaDriver(media.NewLocalDriver(t.TempDir(), "http://example.com"))
	r := chi.NewRouter()
	r.Post("/api/admin/media", h.UploadMedia)
	return r
}

func TestUploadMediaRejectsSVG(t *testing.T) {
	r := newMediaRouter(t)
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`)

	w := uploadMedia(t, r, "evil.svg", svg)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415; body: %s", w.Code, w.Body.String())
	}
}

func TestUploadMediaAcceptsPNG(t *testing.T) {
	r := newMediaRouter(t)
	png, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
	)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	w := uploadMedia(t, r, "pixel.png", png)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestUploadMediaAcceptsWebP(t *testing.T) {
	r := newMediaRouter(t)
	// Minimal RIFF/WEBP header is enough for content detection; the driver
	// stores the bytes as-is.
	webp := append([]byte{0x52, 0x49, 0x46, 0x46}, 0x04, 0x00, 0x00, 0x00)
	webp = append(webp, []byte{'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}...)

	w := uploadMedia(t, r, "pixel.webp", webp)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestUploadMediaAcceptsGIF(t *testing.T) {
	r := newMediaRouter(t)
	gif := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")

	w := uploadMedia(t, r, "pixel.gif", gif)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}
