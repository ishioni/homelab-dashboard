package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"homelab-dashboard/internal/config"
	"homelab-dashboard/internal/dashboard"
)

//go:embed templates/*.gohtml static/*
var assets embed.FS

type dashboardService interface {
	Hub(context.Context) dashboard.ViewModel
	Security(context.Context) dashboard.ViewModel
	Anomalies(context.Context) dashboard.ViewModel
	Forecast(context.Context) dashboard.ViewModel
}

type Server struct {
	cfg       config.Config
	service   dashboardService
	templates *template.Template
	staticFS  http.Handler
}

type pageView struct {
	dashboard.ViewModel
	BodyHTML template.HTML
}

func NewServer(cfg config.Config, service dashboardService) *Server {
	funcs := template.FuncMap{
		"ago": func(value time.Time) string {
			if value.IsZero() {
				return "n/a"
			}
			diff := time.Since(value).Round(time.Minute)
			if diff < time.Minute {
				return "just now"
			}
			return diff.String() + " ago"
		},
		"ts": func(value time.Time) string {
			if value.IsZero() {
				return ""
			}
			return value.UTC().Format(time.RFC3339)
		},
		"meterWidth": func(value float64) string {
			if value < 0 {
				value = 0
			}
			if value > 100 {
				value = 100
			}
			return strings.TrimSpace(strings.TrimRight(strings.TrimRight(
				strconv.FormatFloat(value, 'f', 1, 64), "0"), ".")) + "%"
		},
		"anomalyIcon": func(severity string) string {
			switch severity {
			case "critical":
				return "error"
			case "warning":
				return "warning"
			default:
				return "commit"
			}
		},
		"timelineIcon": func(label string) string {
			switch strings.ToLower(strings.TrimSpace(label)) {
			case "compute":
				return "memory"
			case "network":
				return "route"
			case "storage":
				return "database"
			case "operators":
				return "settings_suggest"
			default:
				return "monitoring"
			}
		},
	}

	tmpl := template.Must(template.New("page.gohtml").Funcs(funcs).ParseFS(assets, "templates/*.gohtml"))
	staticRoot := mustSubFS(assets, "static")

	return &Server{
		cfg:       cfg,
		service:   service,
		templates: tmpl,
		staticFS:  http.FileServer(http.FS(staticRoot)),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", s.staticFS))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", s.wrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Hub(ctx)
	}))
	mux.HandleFunc("/security", s.wrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Security(ctx)
	}))
	mux.HandleFunc("/anomalies", s.wrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Anomalies(ctx)
	}))
	mux.HandleFunc("/forecasting", s.wrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Forecast(ctx)
	}))
	mux.HandleFunc("/api/view/hub", s.apiWrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Hub(ctx)
	}))
	mux.HandleFunc("/api/view/security", s.apiWrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Security(ctx)
	}))
	mux.HandleFunc("/api/view/anomalies", s.apiWrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Anomalies(ctx)
	}))
	mux.HandleFunc("/api/view/forecasting", s.apiWrap(func(ctx context.Context) dashboard.ViewModel {
		return s.service.Forecast(ctx)
	}))

	return mux
}

func (s *Server) wrap(load func(context.Context) dashboard.ViewModel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/security" && r.URL.Path != "/anomalies" && r.URL.Path != "/forecasting" {
			http.NotFound(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.PrometheusTimeout)
		defer cancel()

		model := load(ctx)
		bodyHTML, err := s.renderBody(model)
		if err != nil {
			log.Printf("content render failed: %v", err)
			http.Error(w, "content render failed", http.StatusInternalServerError)
			return
		}

		view := pageView{
			ViewModel: model,
			BodyHTML:  template.HTML(bodyHTML),
		}

		if err := s.templates.ExecuteTemplate(w, "page", view); err != nil {
			log.Printf("template render failed: %v", err)
			http.Error(w, "template render failed", http.StatusInternalServerError)
		}
	}
}

func (s *Server) apiWrap(load func(context.Context) dashboard.ViewModel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.PrometheusTimeout)
		defer cancel()

		model := load(ctx)
		headerHTML, err := s.renderPartial("topbar_meta", model)
		if err != nil {
			log.Printf("header partial render failed: %v", err)
			http.Error(w, "header render failed", http.StatusInternalServerError)
			return
		}

		bodyHTML, err := s.renderBody(model)
		if err != nil {
			log.Printf("content partial render failed: %v", err)
			http.Error(w, "content render failed", http.StatusInternalServerError)
			return
		}

		payload := struct {
			HeaderHTML string `json:"header_html"`
			BodyHTML   string `json:"body_html"`
		}{
			HeaderHTML: headerHTML,
			BodyHTML:   bodyHTML,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			log.Printf("json encode failed: %v", err)
		}
	}
}

func (s *Server) renderBody(model dashboard.ViewModel) (string, error) {
	return s.renderPartial(bodyTemplateName(model.Screen), model)
}

func (s *Server) renderPartial(name string, model dashboard.ViewModel) (string, error) {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, model); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func bodyTemplateName(screen string) string {
	switch screen {
	case "hub":
		return "hub_body"
	case "security":
		return "security_body"
	case "anomalies":
		return "anomalies_body"
	case "forecasting":
		return "forecasting_body"
	default:
		return "hub_body"
	}
}

func mustSubFS(root fs.FS, path string) fs.FS {
	sub, err := fs.Sub(root, path)
	if err != nil {
		panic(err)
	}
	return sub
}
