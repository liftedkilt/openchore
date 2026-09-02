package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const uploadDir = "data/uploads"

func init() {
	_ = os.MkdirAll(uploadDir, 0750)
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

	// Validate MIME type by reading the first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}
	mimeType := http.DetectContentType(buf[:n])
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	if !allowedTypes[mimeType] {
		writeError(w, http.StatusBadRequest, "only image files are allowed (JPEG, PNG, GIF, WebP)")
		return
	}
	// Seek back to the beginning after reading for MIME detection
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to process file")
			return
		}
	}

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

// resolveUploadPath converts a stored photo URL (e.g. "/uploads/123_upload.jpg")
// into a path on disk inside uploadDir. Only plain filenames within the uploads
// directory are accepted, so user-supplied values can't escape it.
func resolveUploadPath(photoURL string) (string, error) {
	name := strings.TrimPrefix(photoURL, "/uploads/")
	if name == photoURL {
		name = strings.TrimPrefix(photoURL, "uploads/")
		if name == photoURL {
			return "", fmt.Errorf("photo url %q is not an upload", photoURL)
		}
	}
	// filepath.Base strips any directory traversal ("../", nested paths).
	name = filepath.Base(filepath.Clean("/" + name))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "", fmt.Errorf("photo url %q has no filename", photoURL)
	}
	return filepath.Join(uploadDir, name), nil
}
