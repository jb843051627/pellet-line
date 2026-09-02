package handler

import (
	"net/http"

	"github.com/jb843051627/pellet-line/internal/service"
)

// handleBatches GET 列表 / POST 建批。
func (s *Server) handleBatches(w http.ResponseWriter, r *http.Request) {
	svc := service.BatchService{App: s.App}
	switch r.Method {
	case http.MethodGet:
		batches, err := svc.ListBatches(r.Context(), r.URL.Query().Get("line"), r.URL.Query().Get("state"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"batches": batches})
	case http.MethodPost:
		var in service.CreateBatchInput
		if !readJSON(w, r, &in) {
			return
		}
		batch, err := svc.CreateBatch(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, batch)
	default:
		methodNotAllowed(w)
	}
}

// handleBatchItem 子资源 POST <action>（start|finish|qc|recheck|close）。
func (s *Server) handleBatchItem(w http.ResponseWriter, r *http.Request) {
	svc := service.BatchService{App: s.App}
	parts := splitPath(r.URL.Path)
	id, action := subResource(parts, "batches")
	if id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "missing batch id"})
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "":
		batch, err := svc.GetBatch(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, batch)
	case r.Method == http.MethodPost && action == "start":
		batch, err := svc.StartBatch(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, batch)
	case r.Method == http.MethodPost && action == "finish":
		var in struct {
			OutputTonnes float64 `json:"output_tonnes"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		batch, err := svc.FinishProduction(r.Context(), id, in.OutputTonnes)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, batch)
	case r.Method == http.MethodPost && action == "qc":
		var in service.QCResult
		if !readJSON(w, r, &in) {
			return
		}
		in.BatchID = id
		batch, err := svc.SubmitQC(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, batch)
	case r.Method == http.MethodPost && action == "recheck":
		batch, err := svc.RecheckQC(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, batch)
	case r.Method == http.MethodPost && action == "close":
		batch, err := svc.CloseBatch(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, batch)
	default:
		methodNotAllowed(w)
	}
}

func subResource(parts []string, key string) (string, string) {
	for i, p := range parts {
		if p == key {
			id := ""
			action := ""
			if i+1 < len(parts) {
				id = parts[i+1]
			}
			if i+2 < len(parts) {
				action = parts[i+2]
			}
			return id, action
		}
	}
	return "", ""
}
