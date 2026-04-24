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

type VolsyncSource struct {
	Name             string
	Namespace        string
	Schedule         string
	SourcePVC        string
	Method           string
	Status           string
	Message          string
	LastResult       string
	LastSyncTime     time.Time
	NextSyncTime     time.Time
	LastSyncDuration time.Duration
	LastTransition   time.Time
}

type ExternalSecret struct {
	Name            string
	Namespace       string
	StoreRef        string
	StoreKind       string
	RefreshInterval time.Duration
	RefreshTime     time.Time
	Status          string
	Message         string
	LastTransition  time.Time
}

type ClusterSecretStore struct {
	Name           string
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

func (c *Client) WarningEvents(ctx context.Context, limit int, allowlist []string) ([]WarningEvent, error) {
	if c == nil || c.clientset == nil {
		return nil, errors.New("kubernetes client unavailable")
	}

	result := make([]WarningEvent, 0, limit)
	namespaces := namespacesForList(allowlist)
	for _, namespace := range namespaces {
		events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: "type=Warning",
		})
		if err != nil {
			return nil, err
		}

		for _, event := range events.Items {
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

	resources := make([]FluxResource, 0, 128)
	for _, gvr := range gvrs {
		for _, namespace := range namespacesForList(allowlist) {
			list, err := c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("list %s.%s: %w", gvr.Resource, gvr.Group, err)
			}

			for _, item := range list.Items {
				resource, ok := fluxResourceFromObject(item)
				if !ok {
					continue
				}
				resources = append(resources, resource)
			}
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

func (c *Client) VolsyncSources(ctx context.Context, limit int, allowlist []string) ([]VolsyncSource, error) {
	if c == nil || c.dynamic == nil {
		return nil, errors.New("kubernetes dynamic client unavailable")
	}

	gvr := schema.GroupVersionResource{
		Group:    "volsync.backube",
		Version:  "v1alpha1",
		Resource: "replicationsources",
	}

	sources := make([]VolsyncSource, 0, 32)
	for _, namespace := range namespacesForList(allowlist) {
		list, err := c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list %s.%s: %w", gvr.Resource, gvr.Group, err)
		}

		for _, item := range list.Items {
			source, ok := volsyncSourceFromObject(item)
			if !ok {
				continue
			}
			sources = append(sources, source)
		}
	}

	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Namespace != sources[j].Namespace {
			return sources[i].Namespace < sources[j].Namespace
		}
		return sources[i].Name < sources[j].Name
	})

	if limit > 0 && len(sources) > limit {
		return sources[:limit], nil
	}

	return sources, nil
}

func (c *Client) ExternalSecrets(ctx context.Context, limit int, allowlist []string) ([]ExternalSecret, error) {
	if c == nil || c.dynamic == nil {
		return nil, errors.New("kubernetes dynamic client unavailable")
	}

	gvr := schema.GroupVersionResource{
		Group:    "external-secrets.io",
		Version:  "v1",
		Resource: "externalsecrets",
	}

	secrets := make([]ExternalSecret, 0, 64)
	for _, namespace := range namespacesForList(allowlist) {
		list, err := c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list %s.%s: %w", gvr.Resource, gvr.Group, err)
		}

		for _, item := range list.Items {
			secret, ok := externalSecretFromObject(item)
			if !ok {
				continue
			}
			secrets = append(secrets, secret)
		}
	}

	sort.Slice(secrets, func(i, j int) bool {
		if secrets[i].Namespace != secrets[j].Namespace {
			return secrets[i].Namespace < secrets[j].Namespace
		}
		return secrets[i].Name < secrets[j].Name
	})

	if limit > 0 && len(secrets) > limit {
		return secrets[:limit], nil
	}

	return secrets, nil
}

func (c *Client) ClusterSecretStores(ctx context.Context) ([]ClusterSecretStore, error) {
	if c == nil || c.dynamic == nil {
		return nil, errors.New("kubernetes dynamic client unavailable")
	}

	gvr := schema.GroupVersionResource{
		Group:    "external-secrets.io",
		Version:  "v1",
		Resource: "clustersecretstores",
	}

	list, err := c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list %s.%s: %w", gvr.Resource, gvr.Group, err)
	}

	stores := make([]ClusterSecretStore, 0, len(list.Items))
	for _, item := range list.Items {
		store, ok := clusterSecretStoreFromObject(item)
		if !ok {
			continue
		}
		stores = append(stores, store)
	}

	sort.Slice(stores, func(i, j int) bool {
		return stores[i].Name < stores[j].Name
	})

	return stores, nil
}

func namespacesForList(allowlist []string) []string {
	seen := map[string]struct{}{}
	namespaces := make([]string, 0, len(allowlist))
	for _, namespace := range allowlist {
		namespace = strings.TrimSpace(namespace)
		if namespace == "" {
			continue
		}
		if _, ok := seen[namespace]; ok {
			continue
		}
		seen[namespace] = struct{}{}
		namespaces = append(namespaces, namespace)
	}
	if len(namespaces) == 0 {
		return []string{metav1.NamespaceAll}
	}
	return namespaces
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

func volsyncSourceFromObject(item unstructured.Unstructured) (VolsyncSource, bool) {
	source := VolsyncSource{
		Name:      item.GetName(),
		Namespace: item.GetNamespace(),
	}

	source.Schedule, _, _ = unstructured.NestedString(item.Object, "spec", "trigger", "schedule")
	source.SourcePVC, _, _ = unstructured.NestedString(item.Object, "spec", "sourcePVC")
	source.Method = volsyncMethod(item.Object)

	lastSyncTime, _, _ := unstructured.NestedString(item.Object, "status", "lastSyncTime")
	if parsed, err := time.Parse(time.RFC3339, lastSyncTime); err == nil {
		source.LastSyncTime = parsed
	}

	nextSyncTime, _, _ := unstructured.NestedString(item.Object, "status", "nextSyncTime")
	if parsed, err := time.Parse(time.RFC3339, nextSyncTime); err == nil {
		source.NextSyncTime = parsed
	}

	lastSyncDuration, _, _ := unstructured.NestedString(item.Object, "status", "lastSyncDuration")
	if parsed, err := time.ParseDuration(lastSyncDuration); err == nil {
		source.LastSyncDuration = parsed
	}

	source.LastResult, _, _ = unstructured.NestedString(item.Object, "status", "latestMoverStatus", "result")

	conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
	for _, rawCondition := range conditions {
		conditionMap, ok := rawCondition.(map[string]any)
		if !ok {
			continue
		}

		conditionType, _ := conditionMap["type"].(string)
		if conditionType != "Reconciled" && conditionType != "Synchronizing" {
			continue
		}

		status, _ := conditionMap["status"].(string)
		reason, _ := conditionMap["reason"].(string)
		message, _ := conditionMap["message"].(string)
		lastTransition, _ := conditionMap["lastTransitionTime"].(string)

		source.Message = message
		source.Status = volsyncSourceStatus(status, reason, source.LastResult)
		if parsed, err := time.Parse(time.RFC3339, lastTransition); err == nil {
			source.LastTransition = parsed
		}
		break
	}

	if source.Status == "" {
		source.Status = volsyncSourceStatus("", "", source.LastResult)
	}

	return source, true
}

func volsyncMethod(object map[string]any) string {
	methods := []struct {
		key   string
		label string
	}{
		{key: "restic", label: "restic"},
		{key: "rclone", label: "rclone"},
		{key: "rsync", label: "rsync"},
		{key: "rsyncTLS", label: "rsync-tls"},
	}

	for _, method := range methods {
		if _, found, _ := unstructured.NestedMap(object, "spec", method.key); found {
			return method.label
		}
	}

	return "volsync"
}

func volsyncSourceStatus(status, reason, lastResult string) string {
	if strings.EqualFold(lastResult, "Failed") {
		return "Failed"
	}

	switch reason {
	case "WaitingForSchedule":
		return "Scheduled"
	case "SyncInProgress", "Reconciling":
		return "Synchronizing"
	case "ReconcileComplete":
		return "Healthy"
	}

	if strings.EqualFold(status, "True") {
		return "Healthy"
	}

	if strings.EqualFold(lastResult, "Successful") {
		return "Healthy"
	}

	return "Pending"
}

func externalSecretFromObject(item unstructured.Unstructured) (ExternalSecret, bool) {
	secret := ExternalSecret{
		Name:      item.GetName(),
		Namespace: item.GetNamespace(),
	}

	secret.StoreRef, _, _ = unstructured.NestedString(item.Object, "spec", "secretStoreRef", "name")
	secret.StoreKind, _, _ = unstructured.NestedString(item.Object, "spec", "secretStoreRef", "kind")

	refreshInterval, _, _ := unstructured.NestedString(item.Object, "spec", "refreshInterval")
	if parsed, err := time.ParseDuration(refreshInterval); err == nil {
		secret.RefreshInterval = parsed
	}

	refreshTime, _, _ := unstructured.NestedString(item.Object, "status", "refreshTime")
	if parsed, err := time.Parse(time.RFC3339, refreshTime); err == nil {
		secret.RefreshTime = parsed
	}

	conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
	for _, rawCondition := range conditions {
		conditionMap, ok := rawCondition.(map[string]any)
		if !ok {
			continue
		}
		conditionType, _ := conditionMap["type"].(string)
		if conditionType != "Ready" {
			continue
		}

		status, _ := conditionMap["status"].(string)
		reason, _ := conditionMap["reason"].(string)
		message, _ := conditionMap["message"].(string)
		lastTransition, _ := conditionMap["lastTransitionTime"].(string)

		secret.Status = externalSecretStatus(status, reason)
		secret.Message = message
		if parsed, err := time.Parse(time.RFC3339, lastTransition); err == nil {
			secret.LastTransition = parsed
		}
		break
	}

	if secret.Status == "" {
		secret.Status = "Pending"
	}

	return secret, true
}

func clusterSecretStoreFromObject(item unstructured.Unstructured) (ClusterSecretStore, bool) {
	store := ClusterSecretStore{
		Name: item.GetName(),
	}

	conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
	for _, rawCondition := range conditions {
		conditionMap, ok := rawCondition.(map[string]any)
		if !ok {
			continue
		}
		conditionType, _ := conditionMap["type"].(string)
		if conditionType != "Ready" {
			continue
		}

		status, _ := conditionMap["status"].(string)
		reason, _ := conditionMap["reason"].(string)
		message, _ := conditionMap["message"].(string)
		lastTransition, _ := conditionMap["lastTransitionTime"].(string)

		store.Status = externalSecretStatus(status, reason)
		store.Message = message
		if parsed, err := time.Parse(time.RFC3339, lastTransition); err == nil {
			store.LastTransition = parsed
		}
		break
	}

	if store.Status == "" {
		store.Status = "Pending"
	}

	return store, true
}

func externalSecretStatus(status, reason string) string {
	if strings.EqualFold(status, "True") {
		return "Ready"
	}
	switch reason {
	case "SecretSynced":
		return "Ready"
	case "SecretSyncedError", "ProviderError", "UpdateFailed":
		return "Error"
	}
	if strings.EqualFold(status, "False") {
		return "Error"
	}
	return "Pending"
}
