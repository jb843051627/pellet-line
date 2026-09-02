package handler

import (
	"net/http"
	"time"

	"github.com/jb843051627/pellet-line/internal/service"
)

// Server HTTP 处理器聚合。
type Server struct {
	App *service.App
}

// NewServer 构造处理器。
func NewServer(app *service.App) *Server {
	return &Server{App: app}
}

// Home 服务健康页。
func (s *Server) Home(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "now": time.Now().Format(time.RFC3339)})
}
