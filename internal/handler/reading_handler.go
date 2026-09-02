package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/jb843051627/pellet-line/internal/service"
)

// handleReadings GET 查询 / POST 单条或批量。
func (s *Server) handleReadings(w http.ResponseWriter, r *http.Request) {
	svc := service.ReadingService{App: s.App}
	switch r.Method {
	case http.MethodGet:
		point := r.URL.Query().Get("point")
		limit := parseQueryLimit(r, "limit", 0)
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		fromT, err := time.Parse(time.RFC3339, from)
		if err != nil {
			writeError(w, err)
			return
		}
		toT, err := time.Parse(time.RFC3339, to)
		if err != nil {
			writeError(w, err)
			return
		}
		items, err := svc.ListReadings(r.Context(), point, fromT, toT, limit)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"readings": items})
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, err)
			return
		}
		trimmed := firstNonSpace(body)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			var batch []service.IngestReadingInput
			if err := json.Unmarshal(body, &batch); err != nil {
				writeError(w, err)
				return
			}
			n, err := svc.IngestReadings(r.Context(), batch)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"inserted": n})
			return
		}
		var single service.IngestReadingInput
		if err := json.Unmarshal(body, &single); err != nil {
			writeError(w, err)
			return
		}
		item, err := svc.IngestReading(r.Context(), single)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func firstNonSpace(b []byte) []byte {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	return b[i:]
}

// handleRecentReadings GET 最近缓存读数（趋势）。
func (s *Server) handleRecentReadings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	point := r.URL.Query().Get("point")
	if point == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "point required"})
		return
	}
	svc := service.ReadingService{App: s.App}
	items := svc.RecentByPoint(point)
	writeJSON(w, http.StatusOK, map[string]any{"point": point, "readings": items})
}
