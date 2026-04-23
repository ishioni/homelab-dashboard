package kube

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	clientset *kubernetes.Clientset
}

type WarningEvent struct {
	Namespace string
	Reason    string
	Message   string
	Object    string
	When      time.Time
}

func NewClient(kubeconfigPath string) (*Client, error) {
	cfg, err := buildConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

func (c *Client) WarningEvents(ctx context.Context, limit int) ([]WarningEvent, error) {
	if c == nil || c.clientset == nil {
		return nil, errors.New("kubernetes client unavailable")
	}

	events, err := c.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]WarningEvent, 0, len(events.Items))
	for _, event := range events.Items {
		if event.Type != corev1.EventTypeWarning {
			continue
		}

		when := event.LastTimestamp.Time
		if when.IsZero() {
			when = event.EventTime.Time
		}
		if when.IsZero() {
			when = event.CreationTimestamp.Time
		}

		result = append(result, WarningEvent{
			Namespace: event.Namespace,
			Reason:    event.Reason,
			Message:   event.Message,
			Object:    fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
			When:      when,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].When.After(result[j].When)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	candidates := []string{}
	if kubeconfigPath != "" {
		candidates = append(candidates, kubeconfigPath)
	}
	if env := os.Getenv("KUBECONFIG"); env != "" {
		candidates = append(candidates, env)
	}
	if home := homedir.HomeDir(); home != "" {
		candidates = append(candidates, filepath.Join(home, ".kube", "config"))
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		cfg, err := clientcmd.BuildConfigFromFlags("", candidate)
		if err == nil {
			return cfg, nil
		}
	}

	return nil, errors.New("no usable kubeconfig or in-cluster configuration found")
}
