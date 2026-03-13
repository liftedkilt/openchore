package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const uploadDir = "data/uploads"

func init() {
	_ = os.MkdirAll(uploadDir, 0755)
}

func (h *ChoreHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 10MB)")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no photo provided")
		return
	}
	defer file.Close()

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg" // Fallback
	}
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), "upload", ext)
	path := filepath.Join(uploadDir, filename)

	out, err := os.Create(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file content")
		return
	}

	// Return the relative URL
	writeJSON(w, http.StatusOK, map[string]string{
		"url": "/uploads/" + filename,
	})
}
