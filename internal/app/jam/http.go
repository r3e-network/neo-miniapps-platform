package jam

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewHTTPHandler returns a ServeMux exposing minimal JAM endpoints.
func NewHTTPHandler(store PackageStore, preimages PreimageStore, coord Coordinator) http.Handler {
	h := &httpHandler{
		store:     store,
		preimages: preimages,
		coord:     coord,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jam/preimages/", h.preimagesHandler)
	mux.HandleFunc("/jam/packages", h.packagesHandler)
	mux.HandleFunc("/jam/packages/", h.packageResource)
	mux.HandleFunc("/jam/process", h.processHandler)
	return mux
}

type httpHandler struct {
	store     PackageStore
	preimages PreimageStore
	coord     Coordinator
}

func (h *httpHandler) preimagesHandler(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, "/jam/preimages/")
	if hash == "" {
		writeError(w, http.StatusBadRequest, Err("missing preimage hash"))
		return
	}
	switch r.Method {
	case http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		mediaType := r.Header.Get("Content-Type")
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		meta, err := h.preimages.Put(r.Context(), hash, mediaType, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, meta)
	case http.MethodGet:
		meta, err := h.preimages.Stat(r.Context(), hash)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		rc, err := h.preimages.Get(r.Context(), hash)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		defer rc.Close()
		w.Header().Set("Content-Type", meta.MediaType)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)
	case http.MethodHead:
		meta, err := h.preimages.Stat(r.Context(), hash)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		w.Header().Set("Content-Type", meta.MediaType)
		if meta.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
		}
		w.Header().Set("X-Preimage-Hash", meta.Hash)
		w.Header().Set("X-Preimage-Size", strconv.FormatInt(meta.Size, 10))
		w.Header().Set("X-Preimage-Media-Type", meta.MediaType)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *httpHandler) packagesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var pkg WorkPackage
		if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if pkg.ID == "" {
			pkg.ID = uuid.NewString()
		}
		if pkg.Status == "" {
			pkg.Status = PackageStatusPending
		}
		if pkg.CreatedAt.IsZero() {
			pkg.CreatedAt = time.Now().UTC()
		}
		for i := range pkg.Items {
			if pkg.Items[i].ID == "" {
				pkg.Items[i].ID = uuid.NewString()
			}
			pkg.Items[i].PackageID = pkg.ID
		}
		if err := h.store.EnqueuePackage(r.Context(), pkg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, pkg)
	case http.MethodGet:
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		pkgs, err := h.store.ListPackages(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, pkgs)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *httpHandler) packageResource(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/jam/packages/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	pkgID := parts[0]

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		pkg, err := h.store.GetPackage(r.Context(), pkgID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, pkg)
		return
	}

	if parts[1] == "report" && r.Method == http.MethodGet {
		report, attns, err := h.store.GetReportByPackage(r.Context(), pkgID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"report":       report,
			"attestations": attns,
		})
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *httpHandler) processHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ok, err := h.coord.ProcessNext(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"processed": true})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
