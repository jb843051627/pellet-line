package handler

import (
	"net/http"

	"github.com/jb843051627/pellet-line/internal/service"
)

// handleLots GET 列表 / POST 登记。
func (s *Server) handleLots(w http.ResponseWriter, r *http.Request) {
	svc := service.LotService{App: s.App}
	switch r.Method {
	case http.MethodGet:
		lots, err := svc.ListLots(r.Context(), r.URL.Query().Get("supplier"), r.URL.Query().Get("state"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"lots": lots})
	case http.MethodPost:
		var in service.RegisterLotInput
		if !readJSON(w, r, &in) {
			return
		}
		lot, err := svc.RegisterLot(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, lot)
	default:
		methodNotAllowed(w)
	}
}

// handleLotItem 子资源：GET 详情 / POST <action>（assay|receive）。
func (s *Server) handleLotItem(w http.ResponseWriter, r *http.Request) {
	svc := service.LotService{App: s.App}
	parts := splitPath(r.URL.Path)
	id := ""
	action := ""
	for i, p := range parts {
		if p == "lots" {
			if i+1 < len(parts) {
				id = parts[i+1]
			}
			if i+2 < len(parts) {
				action = parts[i+2]
			}
		}
	}
	if id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "missing lot id"})
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "":
		lot, err := svc.GetLot(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, lot)
	case r.Method == http.MethodPost && action == "assay":
		var in service.AssayLotInput
		if !readJSON(w, r, &in) {
			return
		}
		in.LotID = id
		lot, err := svc.AssayLot(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, lot)
	case r.Method == http.MethodPost && action == "receive":
		lot, err := svc.ReceiveLot(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, lot)
	default:
		methodNotAllowed(w)
	}
}
