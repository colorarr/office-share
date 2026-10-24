package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"office-share/internal/app"
)

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func main() {
	a, err := app.New(env("DATA_DIR", "./data"), env("WEB_DIR", "./web/dist"), env("BASE_URL", "http://127.0.0.1:8080"), os.Getenv("MYSQL_DSN"), os.Getenv("ADMIN_PASSWORD"))
	if err != nil {
		log.Fatal(err)
	}
	defer a.DB.Close()
	server := &http.Server{Addr: env("LISTEN_ADDR", "127.0.0.1:8080"), Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Minute, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("graceful shutdown: %v", err)
			_ = server.Close()
		}
	}()
	log.Printf("office-share listening on %s", server.Addr)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-shutdownDone
}
