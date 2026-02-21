package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ErrCreateKubeConfig    = errors.New("failed to create kubernetes config")
	ErrCreateKubeClientset = errors.New("failed to create kubernetes clientset")
	ErrResourceFetch       = errors.New("failed to fetch resource")
	ErrHealthCheckFailed   = errors.New("health check failed")
)

type Client struct {
	clientset *clientset.Clientset
	config    *rest.Config
	logger    *slog.Logger
}

func (c *Client) GetRawInterface() clientset.Interface {
	return c.clientset
}

func NewClient(logger *slog.Logger) (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCreateKubeConfig, err)
		}
	}

	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateKubeClientset, err)
	}

	return &Client{
		clientset: cs,
		config:    config,
		logger:    logger,
	}, nil
}

func listResource[L any, I any, O any](
	ctx context.Context,
	fetcher func(context.Context, metav1.ListOptions) (*L, error),
	itemsAccessor func(*L) []I,
	mapper func(*I) O,
) ([]O, error) {
	list, err := fetcher(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	items := itemsAccessor(list)
	out := make([]O, len(items))
	for i := range items {
		out[i] = mapper(&items[i])
	}
	return out, nil
}

func getResource[I any, O any](
	ctx context.Context,
	fetcher func(context.Context, string, metav1.GetOptions) (*I, error),
	name string,
	mapper func(*I) O,
) (*O, error) {
	item, err := fetcher(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	out := mapper(item)
	return &out, nil
}

func (c *Client) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	return listResource(
		ctx,
		c.clientset.CoreV1().Nodes().List,
		func(l *corev1.NodeList) []corev1.Node { return l.Items },
		MapNode,
	)
}

func (c *Client) GetNode(ctx context.Context, nodeName string) (*NodeInfo, error) {
	return getResource(ctx, c.clientset.CoreV1().Nodes().Get, nodeName, MapNode)
}

func (c *Client) GetPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	return listResource(
		ctx,
		c.clientset.CoreV1().Pods(namespace).List,
		func(l *corev1.PodList) []corev1.Pod { return l.Items },
		MapPod,
	)
}

func (c *Client) GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error) {
	return c.GetPods(ctx, "")
}

func (c *Client) GetPod(ctx context.Context, namespace, name string) (*PodInfo, error) {
	return getResource(ctx, c.clientset.CoreV1().Pods(namespace).Get, name, MapPod)
}

func (c *Client) GetServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	return listResource(
		ctx,
		c.clientset.CoreV1().Services(namespace).List,
		func(l *corev1.ServiceList) []corev1.Service { return l.Items },
		MapService,
	)
}

func (c *Client) GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error) {
	return c.GetServices(ctx, "")
}

func (c *Client) GetEvents(ctx context.Context, namespace string) ([]EventInfo, error) {
	return listResource(
		ctx,
		c.clientset.CoreV1().Events(namespace).List,
		func(l *corev1.EventList) []corev1.Event { return l.Items },
		MapEvent,
	)
}

func (c *Client) GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error) {
	return c.GetEvents(ctx, "")
}

func (c *Client) GetNamespaces(ctx context.Context) ([]string, error) {
	ns, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ns.Items))
	for i, item := range ns.Items {
		names[i] = item.Name
	}
	return names, nil
}

func (c *Client) GetResourceQuotas(ctx context.Context, namespace string) ([]ResourceQuotaInfo, error) {
	return listResource(
		ctx,
		c.clientset.CoreV1().ResourceQuotas(namespace).List,
		func(l *corev1.ResourceQuotaList) []corev1.ResourceQuota { return l.Items },
		MapResourceQuota,
	)
}

func (c *Client) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	nodes, _ := c.GetNodes(ctx)
	pods, _ := c.GetPodsAllNamespaces(ctx)
	namespaces, _ := c.GetNamespaces(ctx)

	return &ClusterInfo{
		Version:    version.GitVersion,
		NodeCount:  len(nodes),
		PodCount:   len(pods),
		Namespaces: namespaces,
		APIVersion: fmt.Sprintf("%s.%s", version.Major, version.Minor),
		ServerURL:  c.config.Host,
	}, nil
}

func (c *Client) GetResource(ctx context.Context, namespace, resource string) ([]runtime.Object, error) {
	return nil, errors.New("generic GetResource requires dynamic client implementation")
}

func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.clientset.Discovery().ServerVersion()
	return err
}

func MapNode(node *corev1.Node) NodeInfo {
	conditions := make([]NodeCondition, len(node.Status.Conditions))
	for i, cond := range node.Status.Conditions {
		conditions[i] = NodeCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		}
	}

	capacity := make(map[string]string)
	for k, v := range node.Status.Capacity {
		capacity[string(k)] = v.String()
	}

	status := "Unknown"
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			status = "Ready"
			break
		}
	}

	return NodeInfo{
		Name:        node.Name,
		Status:      status,
		Conditions:  conditions,
		Taints:      node.Spec.Taints,
		Capacity:    capacity,
		Age:         time.Since(node.CreationTimestamp.Time),
		Labels:      node.Labels,
		Annotations: node.Annotations,
	}
}

func MapPod(pod *corev1.Pod) PodInfo {
	containers := make([]ContainerInfo, len(pod.Spec.Containers))
	restarts := 0

	for i, container := range pod.Spec.Containers {
		info := ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		}

		if i < len(pod.Status.ContainerStatuses) {
			status := pod.Status.ContainerStatuses[i]
			info.Ready = status.Ready
			info.RestartCount = status.RestartCount
			restarts += int(status.RestartCount)

			switch {
			case status.State.Running != nil:
				info.State = "Running"
			case status.State.Waiting != nil:
				info.State = "Waiting"
			case status.State.Terminated != nil:
				info.State = "Terminated"
			}
		}
		containers[i] = info
	}

	return PodInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		Status:       string(pod.Status.Phase),
		Phase:        pod.Status.Phase,
		NodeName:     pod.Spec.NodeName,
		Age:          time.Since(pod.CreationTimestamp.Time),
		Labels:       pod.Labels,
		Containers:   containers,
		RestartCount: restarts,
	}
}

func MapService(svc *corev1.Service) ServiceInfo {
	return ServiceInfo{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Type:      svc.Spec.Type,
		ClusterIP: svc.Spec.ClusterIP,
		Ports:     svc.Spec.Ports,
		Age:       time.Since(svc.CreationTimestamp.Time),
	}
}

func MapEvent(event *corev1.Event) EventInfo {
	return EventInfo{
		Type:       event.Type,
		Reason:     event.Reason,
		ObjectKind: event.InvolvedObject.Kind,
		ObjectName: event.InvolvedObject.Name,
		Message:    event.Message,
		Count:      event.Count,
		Namespace:  event.Namespace,
	}
}

func MapResourceQuota(rq *corev1.ResourceQuota) ResourceQuotaInfo {
	hard := make(map[string]string)
	for k, v := range rq.Status.Hard {
		hard[string(k)] = v.String()
	}
	used := make(map[string]string)
	for k, v := range rq.Status.Used {
		used[string(k)] = v.String()
	}
	return ResourceQuotaInfo{
		Name:      rq.Name,
		Namespace: rq.Namespace,
		Hard:      hard,
		Used:      used,
	}
}
