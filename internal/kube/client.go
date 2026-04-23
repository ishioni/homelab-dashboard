package kube

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	clientset *kubernetes.Clientset
	dynamic   dynamic.Interface
}

type WarningEvent struct {
	Namespace string
	Reason    string
	Message   string
	Object    string
	When      time.Time
}

type FluxResource struct {
	Kind           string
	Namespace      string
	Name           string
	Ready          bool
	Suspended      bool
	Status         string
	Message        string
	LastTransition time.Time
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

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}

	return &Client{
		clientset: clientset,
		dynamic:   dyn,
	}, nil
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

func (c *Client) FluxResources(ctx context.Context, limit int, allowlist []string) ([]FluxResource, error) {
	if c == nil || c.dynamic == nil {
		return nil, errors.New("kubernetes dynamic client unavailable")
	}

	gvrs := []schema.GroupVersionResource{
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories"},
	}

	allowed := map[string]struct{}{}
	for _, namespace := range allowlist {
		namespace = strings.TrimSpace(namespace)
		if namespace != "" {
			allowed[namespace] = struct{}{}
		}
	}

	resources := make([]FluxResource, 0, 128)
	for _, gvr := range gvrs {
		list, err := c.dynamic.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list %s.%s: %w", gvr.Resource, gvr.Group, err)
		}

		for _, item := range list.Items {
			resource, ok := fluxResourceFromObject(item)
			if !ok {
				continue
			}
			if len(allowed) > 0 {
				if _, ok := allowed[resource.Namespace]; !ok {
					continue
				}
			}
			resources = append(resources, resource)
		}
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].LastTransition.After(resources[j].LastTransition)
	})

	if limit > 0 && len(resources) > limit {
		return resources[:limit], nil
	}

	return resources, nil
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

func fluxResourceFromObject(item unstructured.Unstructured) (FluxResource, bool) {
	resource := FluxResource{
		Kind:      item.GetKind(),
		Namespace: item.GetNamespace(),
		Name:      item.GetName(),
	}

	suspended, _, _ := unstructured.NestedBool(item.Object, "spec", "suspend")
	resource.Suspended = suspended

	conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
	for _, rawCondition := range conditions {
		conditionMap, ok := rawCondition.(map[string]any)
		if !ok {
			continue
		}
		if value, _ := conditionMap["type"].(string); value != "Ready" {
			continue
		}

		status, _ := conditionMap["status"].(string)
		reason, _ := conditionMap["reason"].(string)
		message, _ := conditionMap["message"].(string)
		lastTransition, _ := conditionMap["lastTransitionTime"].(string)

		resource.Ready = strings.EqualFold(status, "True")
		resource.Message = message
		resource.Status = fluxResourceStatus(resource.Suspended, status, reason)
		if parsed, err := time.Parse(time.RFC3339, lastTransition); err == nil {
			resource.LastTransition = parsed
		}
		return resource, true
	}

	if resource.Suspended {
		resource.Status = "Suspended"
	}
	return resource, true
}

func fluxResourceStatus(suspended bool, status, reason string) string {
	if suspended {
		return "Suspended"
	}
	if strings.EqualFold(status, "True") {
		return "Ready"
	}
	switch reason {
	case "Progressing", "Reconciling":
		return "Reconciling"
	case "VerificationError", "ArtifactFailed", "DependencyNotReady", "HealthCheckFailed":
		return "Drift"
	}
	if strings.EqualFold(status, "False") {
		return "Drift"
	}
	return "Pending"
}
