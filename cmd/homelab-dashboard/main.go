package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homelab-dashboard/internal/config"
	"homelab-dashboard/internal/dashboard"
	"homelab-dashboard/internal/kube"
	"homelab-dashboard/internal/prom"
	"homelab-dashboard/internal/web"
)

func main() {
	cfg := config.Load()

	promClient := prom.NewClient(cfg.PrometheusURL, cfg.PrometheusTimeout)
	kubeClient, kubeErr := kube.NewClient(cfg.KubeconfigPath)
	if kubeErr != nil {
		log.Printf("kubernetes client disabled: %v", kubeErr)
	}

	service := dashboard.NewService(cfg, promClient, kubeClient)
	handler := web.NewServer(cfg, service)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown failed: %v", err)
		}
	}()

	log.Printf("starting %s on :%s", cfg.AppName, cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
