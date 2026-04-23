package dashboard

import (
	"strings"
	"testing"

	"homelab-dashboard/internal/config"
)

func TestProjectSeries(t *testing.T) {
	t.Run("empty series", func(t *testing.T) {
		got := projectSeries(nil)
		if got.summary != "insufficient data" {
			t.Fatalf("expected insufficient data summary, got %q", got.summary)
		}
		if got.projected != 0 {
			t.Fatalf("expected zero projection, got %f", got.projected)
		}
	})

	t.Run("rising series", func(t *testing.T) {
		got := projectSeries([]float64{10, 12, 14, 16})
		if got.projected <= 16 {
			t.Fatalf("expected projection above latest value, got %f", got.projected)
		}
		if !strings.Contains(got.summary, "rising") {
			t.Fatalf("expected rising summary, got %q", got.summary)
		}
	})

	t.Run("falling series", func(t *testing.T) {
		got := projectSeries([]float64{16, 12, 8, 4})
		if got.projected < 0 {
			t.Fatalf("expected non-negative projection, got %f", got.projected)
		}
		if !strings.Contains(got.summary, "falling") {
			t.Fatalf("expected falling summary, got %q", got.summary)
		}
	})
}

func TestSparklinePath(t *testing.T) {
	got := sparklinePath([]float64{10, 20, 15, 25}, 180, 56)
	if got == "" {
		t.Fatal("expected non-empty sparkline path")
	}
	if !strings.HasPrefix(got, "M ") {
		t.Fatalf("expected path to start with move command, got %q", got)
	}
	if !strings.Contains(got, "L ") {
		t.Fatalf("expected path to contain line commands, got %q", got)
	}
}

func TestBuildSparkline(t *testing.T) {
	got := buildSparkline("CPU", []float64{40, 45, 50}, "cluster cpu", 85)
	if got.Label != "CPU" {
		t.Fatalf("expected label to be preserved, got %q", got.Label)
	}
	if got.Path == "" {
		t.Fatal("expected sparkline path to be populated")
	}
	if got.Latest != "50.0" {
		t.Fatalf("expected latest value 50.0, got %q", got.Latest)
	}
	if got.Tone != "good" {
		t.Fatalf("expected good tone, got %q", got.Tone)
	}
}

func TestSeverityHelpers(t *testing.T) {
	signals := []AnomalySignal{
		{Severity: "critical"},
		{Severity: "warning"},
		{Severity: "critical"},
	}

	if got := countSeverity(signals, "critical"); got != 2 {
		t.Fatalf("expected 2 critical signals, got %d", got)
	}

	if got := anomalyBannerLabel(signals); got != "Critical Signals Active" {
		t.Fatalf("unexpected anomaly banner label: %q", got)
	}

	if got := anomalyBannerTone(signals); got != "critical" {
		t.Fatalf("unexpected anomaly banner tone: %q", got)
	}
}

func TestParsePercentage(t *testing.T) {
	if got := parsePercentage("88.4%"); got != 88.4 {
		t.Fatalf("expected 88.4, got %f", got)
	}

	if got := parsePercentage("bogus"); got != 0 {
		t.Fatalf("expected invalid percentage to coerce to 0, got %f", got)
	}
}

func TestLoadSharedDemoMode(t *testing.T) {
	service := &Service{cfg: config.Config{DemoMode: true}}
	data, errs := service.loadShared(t.Context())
	if len(errs) != 0 {
		t.Fatalf("expected no backend notes in demo mode, got %d", len(errs))
	}
	if data.fluxTotal == 0 || len(data.anomalies) == 0 {
		t.Fatal("expected populated demo data")
	}
}
