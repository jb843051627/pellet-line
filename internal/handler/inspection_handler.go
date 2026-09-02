package handler

import (
	"net/http"

	"github.com/jb843051627/pellet-line/internal/service"
)

// handleInspections GET 列表 / POST 计划。
func (s *Server) handleInspections(w http.ResponseWriter, r *http.Request) {
	svc := service.InspectionService{App: s.App}
	switch r.Method {
	case http.MethodGet:
		items, err := svc.ListInspections(r.Context(), r.URL.Query().Get("state"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"inspections": items})
	case http.MethodPost:
		var in service.PlanInspectionInput
		if !readJSON(w, r, &in) {
			return
		}
		insp, err := svc.PlanInspection(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, insp)
	default:
		methodNotAllowed(w)
	}
}

// handleEquipment GET 列表 / POST 注册设备。
func (s *Server) handleEquipment(w http.ResponseWriter, r *http.Request) {
	svc := service.EquipmentService{App: s.App}
	switch r.Method {
	case http.MethodGet:
		items, err := svc.List(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"equipment": items})
	case http.MethodPost:
		var in service.RegisterInput
		if !readJSON(w, r, &in) {
			return
		}
		e, err := svc.Register(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, e)
	default:
		methodNotAllowed(w)
	}
}

// handleEquipmentItem GET 单个 / POST <action>（service|hours）。
func (s *Server) handleEquipmentItem(w http.ResponseWriter, r *http.Request) {
	svc := service.EquipmentService{App: s.App}
	parts := splitPath(r.URL.Path)
	code, action := subResource(parts, "equipment")
	if code == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "missing equipment code"})
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "":
		v, err := svc.View(r.Context(), code)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	case r.Method == http.MethodPost && action == "service":
		v, err := svc.PerformService(r.Context(), code)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	case r.Method == http.MethodPost && action == "hours":
		var in struct {
			Hours float64 `json:"hours"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		if err := svc.ReportHours(r.Context(), code, in.Hours); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		methodNotAllowed(w)
	}
}

// handleDashboard GET 看板汇总。
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	svc := service.ReportService{App: s.App}
	sum, err := svc.BuildDashboard(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// handleReport GET 班次报告。
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	svc := service.ReportService{App: s.App}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	fromT, err := parseTimeRFC3339(from, timeParseDefault)
	if err != nil {
		writeError(w, err)
		return
	}
	toT, err := parseTimeRFC3339(to, timeParseDefault)
	if err != nil {
		writeError(w, err)
		return
	}
	rep, err := svc.BuildShiftReport(r.Context(), fromT, toT)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
