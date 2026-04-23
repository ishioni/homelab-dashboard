package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName            string
	ClusterName        string
	Port               string
	PrometheusURL      string
	PrometheusTimeout  time.Duration
	RefreshInterval    time.Duration
	KubeconfigPath     string
	NamespaceAllowlist []string
	EnableKubernetes   bool
	Thresholds         Thresholds
}

type Thresholds struct {
	NodeCPUWarnPercent      float64
	NodeMemoryWarnPercent   float64
	RestartBurstThreshold   float64
	UnavailableTargetsWarn  float64
	FluxUnreadyWarnCount    float64
}

func Load() Config {
	return Config{
		AppName:           getEnv("APP_NAME", "Homelab Dashboard"),
		ClusterName:       getEnv("CLUSTER_NAME", "Homelab Cluster"),
		Port:              getEnv("PORT", "8080"),
		PrometheusURL:     getEnv("PROMETHEUS_URL", "http://thanos-query.monitor.svc.cluster.local:10902"),
		PrometheusTimeout: getDurationEnv("PROMETHEUS_TIMEOUT", 10*time.Second),
		RefreshInterval:   getDurationEnv("REFRESH_INTERVAL", 45*time.Second),
		KubeconfigPath:    os.Getenv("KUBECONFIG"),
		NamespaceAllowlist: splitCSV(
			getEnv("NAMESPACE_ALLOWLIST", ""),
		),
		EnableKubernetes: getBoolEnv("ENABLE_KUBERNETES", true),
		Thresholds: Thresholds{
			NodeCPUWarnPercent:     getFloatEnv("ANOMALY_NODE_CPU_WARN_PERCENT", 85),
			NodeMemoryWarnPercent:  getFloatEnv("ANOMALY_NODE_MEMORY_WARN_PERCENT", 90),
			RestartBurstThreshold:  getFloatEnv("ANOMALY_RESTART_BURST_THRESHOLD", 3),
			UnavailableTargetsWarn: getFloatEnv("ANOMALY_TARGETS_DOWN_WARN_COUNT", 1),
			FluxUnreadyWarnCount:   getFloatEnv("ANOMALY_FLUX_UNREADY_WARN_COUNT", 1),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return duration
}

func getFloatEnv(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func getBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	rawParts := strings.Split(value, ",")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}

	return parts
}
