package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/srmdn/foliocms/internal/media"
)

const maxUploadSize = 10 << 20 // 10 MB

var allowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// SetMediaDriver wires the MediaDriver into the Handler after construction.
func (h *Handler) SetMediaDriver(d media.MediaDriver) {
	h.mediaDriver = d
}

// UploadMedia handles multipart file uploads (POST /api/admin/media).
func (h *Handler) UploadMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	// Detect content type from first 512 bytes.
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])

	if !allowedMIMETypes[contentType] {
		writeError(w, http.StatusUnsupportedMediaType, "only image files are allowed")
		return
	}

	// Seek back so driver reads the full file.
	if _, err := file.Seek(0, 0); err != nil {
		writeError(w, http.StatusInternalServerError, "could not process file")
		return
	}

	key, size, err := h.mediaDriver.Upload(header.Filename, file, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save file")
		return
	}

	now := time.Now().UTC()
	if _, err := h.db.Exec(
		`INSERT INTO media (key, filename, content_type, size, created_at) VALUES (?, ?, ?, ?, ?)`,
		key, header.Filename, contentType, size, now,
	); err != nil {
		// Best-effort cleanup of orphaned file.
		h.mediaDriver.Delete(key)
		writeError(w, http.StatusInternalServerError, "could not record upload")
		return
	}

	writeJSON(w, http.StatusCreated, media.MediaFile{
		Key:         key,
		Filename:    header.Filename,
		ContentType: contentType,
		Size:        size,
		URL:         h.mediaDriver.PublicURL(key),
		CreatedAt:   now,
	})
}

// ListMedia returns all uploaded files (GET /api/admin/media).
func (h *Handler) ListMedia(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(
		`SELECT key, filename, content_type, size, created_at FROM media ORDER BY created_at DESC`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list media")
		return
	}
	defer rows.Close()

	var files []media.MediaFile
	for rows.Next() {
		var f media.MediaFile
		if err := rows.Scan(&f.Key, &f.Filename, &f.ContentType, &f.Size, &f.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read media")
			return
		}
		f.URL = h.mediaDriver.PublicURL(f.Key)
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not read media")
		return
	}

	if files == nil {
		files = []media.MediaFile{}
	}
	writeJSON(w, http.StatusOK, files)
}

// DeleteMedia removes an uploaded file (DELETE /api/admin/media/{key}).
func (h *Handler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing key")
		return
	}

	if err := h.mediaDriver.Delete(key); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file")
		return
	}

	if _, err := h.db.Exec(`DELETE FROM media WHERE key = ?`, key); err != nil {
		writeError(w, http.StatusInternalServerError, "could not remove media record")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
