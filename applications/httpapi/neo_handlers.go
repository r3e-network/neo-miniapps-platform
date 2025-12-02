package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

func (h *handler) neoStatus(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	status, err := h.neo.Status(r.Context())
	if err != nil {
		core.WriteError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// checkpoint is a convenience alias for status, to expose a stable-height style endpoint.
func (h *handler) neoCheckpoint(w http.ResponseWriter, r *http.Request) {
	h.neoStatus(w, r)
}

func (h *handler) neoBlocks(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 20, 200)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0, 10_000_000)
	blocks, err := h.neo.ListBlocks(r.Context(), limit, offset)
	if err != nil {
		core.WriteError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, blocks)
}

func (h *handler) neoBlock(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	path := trimPath(r.URL.Path, "/neo/blocks")
	if path == "" {
		core.WriteError(w, http.StatusBadRequest, ErrMissingHeight)
		return
	}
	height, err := strconv.ParseInt(path, 10, 64)
	if err != nil || height < 0 {
		core.WriteError(w, http.StatusBadRequest, ErrInvalidHeight)
		return
	}
	block, err := h.neo.GetBlock(r.Context(), height)
	if err != nil {
		status := http.StatusBadGateway
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, block)
}

func (h *handler) neoSnapshots(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50, 500)
	snaps, err := h.neo.ListSnapshots(r.Context(), limit)
	if err != nil {
		core.WriteError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

func (h *handler) neoSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	path := trimPath(r.URL.Path, "/neo/snapshots")
	if path == "" {
		core.WriteError(w, http.StatusBadRequest, ErrMissingHeight)
		return
	}
	parts := strings.Split(path, "/")
	heightStr := parts[0]
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil || height < 0 {
		core.WriteError(w, http.StatusBadRequest, ErrInvalidHeight)
		return
	}

	// Snapshot bundle download endpoints:
	if len(parts) == 2 {
		switch parts[1] {
		case "kv":
			h.serveSnapshotBundle(w, r, height, false)
			return
		case "kv-diff":
			h.serveSnapshotBundle(w, r, height, true)
			return
		}
	}

	snap, err := h.neo.GetSnapshot(r.Context(), height)
	if err != nil {
		status := http.StatusBadGateway
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (h *handler) neoStorage(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	path := trimPath(r.URL.Path, "/neo/storage")
	if path == "" {
		core.WriteError(w, http.StatusBadRequest, ErrMissingHeight)
		return
	}
	height, err := strconv.ParseInt(path, 10, 64)
	if err != nil || height < 0 {
		core.WriteError(w, http.StatusBadRequest, ErrInvalidHeight)
		return
	}
	items, err := h.neo.ListStorage(r.Context(), height)
	if err != nil {
		status := http.StatusBadGateway
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *handler) neoStorageDiff(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	path := trimPath(r.URL.Path, "/neo/storage-diff")
	if path == "" {
		core.WriteError(w, http.StatusBadRequest, ErrMissingHeight)
		return
	}
	height, err := strconv.ParseInt(path, 10, 64)
	if err != nil || height < 0 {
		core.WriteError(w, http.StatusBadRequest, ErrInvalidHeight)
		return
	}
	items, err := h.neo.ListStorageDiff(r.Context(), height)
	if err != nil {
		status := http.StatusBadGateway
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *handler) neoStorageSummary(w http.ResponseWriter, r *http.Request) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	path := trimPath(r.URL.Path, "/neo/storage-summary")
	if path == "" {
		core.WriteError(w, http.StatusBadRequest, ErrMissingHeight)
		return
	}
	height, err := strconv.ParseInt(path, 10, 64)
	if err != nil || height < 0 {
		core.WriteError(w, http.StatusBadRequest, ErrInvalidHeight)
		return
	}
	items, err := h.neo.StorageSummary(r.Context(), height)
	if err != nil {
		status := http.StatusBadGateway
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func parseIntDefault(val string, def int, max int) int {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		n = def
	}
	if max > 0 && n > max {
		return max
	}
	return n
}

func trimPath(path, prefix string) string {
	trim := strings.TrimPrefix(path, prefix)
	trim = strings.Trim(trim, "/")
	return trim
}

func (h *handler) serveSnapshotBundle(w http.ResponseWriter, r *http.Request, height int64, diff bool) {
	if h.neo == nil {
		core.WriteError(w, http.StatusServiceUnavailable, ErrNeoUnavailable)
		return
	}
	path, err := h.neo.SnapshotBundlePath(r.Context(), height, diff)
	if err != nil {
		status := http.StatusBadGateway
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		status := http.StatusBadGateway
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	defer file.Close()

	stat, _ := file.Stat()
	name := stat.Name()
	if diff {
		name = "kv-diff-" + name
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	if stat != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	}
	if _, err := io.Copy(w, file); err != nil {
		// response already started; best effort log
		return
	}
}
