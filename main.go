package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jb843051627/pellet-line/internal/handler"
	"github.com/jb843051627/pellet-line/internal/service"
	"github.com/jb843051627/pellet-line/internal/store"
	"github.com/jb843051627/pellet-line/internal/worker"
)

var version = "dev"

func main() {
	var (
		addr  string
		db    string
		showV bool
	)
	flag.StringVar(&addr, "addr", envOr("PELLET_LINE_ADDR", ":8080"), "HTTP listen address")
	flag.StringVar(&db, "db", envOr("PELLET_LINE_DB", "data/pellet-line.db"), "SQLite database path")
	flag.BoolVar(&showV, "version", false, "print version and exit")
	flag.Parse()
	if showV {
		log.Printf("pellet-line %s", version)
		return
	}

	repository, err := store.Open(db)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer repository.Close()

	app := service.NewApp(repository)
	defer app.Shutdown()

	scheduler := worker.NewScheduler(app)
	scheduler.Start()
	defer scheduler.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.HTTPStop(shutdownCtx)
	}()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler.NewRouter(app),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("pellet-line %s listening on %s (db=%s)", version, addr, db)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
