package handler

import (
	"net/http"

	"github.com/jb843051627/pellet-line/internal/service"
	"github.com/jb843051627/pellet-line/internal/web"
)

// NewRouter 构建路由：JSON API + 前端看板静态资源。
func NewRouter(app *service.App) http.Handler {
	srv := NewServer(app)
	mux := http.NewServeMux()
	mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(web.Static()))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/web/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/health", srv.Home)
	mux.HandleFunc("/api/lots", func(w http.ResponseWriter, r *http.Request) { srv.handleLots(w, r) })
	mux.HandleFunc("/api/lots/", func(w http.ResponseWriter, r *http.Request) { srv.handleLotItem(w, r) })
	mux.HandleFunc("/api/batches", func(w http.ResponseWriter, r *http.Request) { srv.handleBatches(w, r) })
	mux.HandleFunc("/api/batches/", func(w http.ResponseWriter, r *http.Request) { srv.handleBatchItem(w, r) })
	mux.HandleFunc("/api/readings", func(w http.ResponseWriter, r *http.Request) { srv.handleReadings(w, r) })
	mux.HandleFunc("/api/readings/recent", func(w http.ResponseWriter, r *http.Request) { srv.handleRecentReadings(w, r) })
	mux.HandleFunc("/api/inspections", func(w http.ResponseWriter, r *http.Request) { srv.handleInspections(w, r) })
	mux.HandleFunc("/api/equipment", func(w http.ResponseWriter, r *http.Request) { srv.handleEquipment(w, r) })
	mux.HandleFunc("/api/equipment/", func(w http.ResponseWriter, r *http.Request) { srv.handleEquipmentItem(w, r) })
	mux.HandleFunc("/api/dashboard", func(w http.ResponseWriter, r *http.Request) { srv.handleDashboard(w, r) })
	mux.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) { srv.handleReport(w, r) })
	return mux
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
