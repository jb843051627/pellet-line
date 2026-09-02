package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jb843051627/pellet-line/internal/model"
)

// writeJSON 统一 JSON 输出。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError 依据错误哨兵映射 HTTP 状态。
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrConflict), errors.Is(err, model.ErrDuplicate):
		status = http.StatusConflict
	case errors.Is(err, model.ErrInvalidInput):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, model.ErrStateTransition):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// readJSON 解析请求体为 v。
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, err)
		return false
	}
	return true
}

// parseIDPath 读取路径参数。
func parseIDPath(r *http.Request, key string) string {
	path := r.URL.Path
	parts := splitPath(path)
	for i, p := range parts {
		if p == key && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func splitPath(path string) []string {
	var out []string
	cur := ""
	for _, c := range path {
		if c == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// parseQueryLimit 解析 limit（默认 0 表示不限）。
func parseQueryLimit(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	return n
}
