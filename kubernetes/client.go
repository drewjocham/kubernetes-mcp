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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ErrCreateKubeConfig    = errors.New("failed to create kubernetes config")
	ErrCreateKubeClientset = errors.New("failed to create kubernetes clientset")
	ErrListNodes           = errors.New("failed to list nodes")
	ErrGetNode             = errors.New("failed to get node")
	ErrListPods            = errors.New("failed to list pods")
	ErrGetPod              = errors.New("failed to get pod")
	ErrListServices        = errors.New("failed to list services")
	ErrListEvents          = errors.New("failed to list events")
	ErrListNamespaces      = errors.New("failed to list namespaces")
	ErrListResourceQuotas  = errors.New("failed to list resource quotas")
	ErrGetServerVersion    = errors.New("failed to get server version")
	ErrNotImplemented      = errors.New("generic resource retrieval not implemented yet")
	ErrHealthCheckFailed   = errors.New("health check failed")
)

type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	logger    *slog.Logger
}

func NewClient(logger *slog.Logger) (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCreateKubeConfig, err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateKubeClientset, err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
		logger:    logger,
	}, nil
}

func NewClientFromConfig(kubeconfigPath string, logger *slog.Logger) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("%w from %s: %w", ErrCreateKubeConfig, kubeconfigPath, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateKubeClientset, err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
		logger:    logger,
	}, nil
}

func (c *Client) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing nodes", "error", err)
		return nil, ErrListNodes
	}

	var nodeInfos []NodeInfo
	for _, node := range nodes.Items {
		nodeInfo := c.convertNodeToNodeInfo(&node)
		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

func (c *Client) GetNode(ctx context.Context, nodeName string) (*NodeInfo, error) {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		c.logger.Error("Error getting node", "node", nodeName, "error", err)
		return nil, fmt.Errorf("%w: %s", ErrGetNode, nodeName)
	}

	nodeInfo := c.convertNodeToNodeInfo(node)
	return &nodeInfo, nil
}

func (c *Client) GetPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing pods", "namespace", namespace, "error", err)
		return nil, fmt.Errorf("%w in namespace %s", ErrListPods, namespace)
	}

	var podInfos []PodInfo
	for _, pod := range pods.Items {
		podInfo := c.convertPodToPodInfo(&pod)
		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

func (c *Client) GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error) {
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing pods from all namespaces", "error", err)
		return nil, ErrListPods
	}

	var podInfos []PodInfo
	for _, pod := range pods.Items {
		podInfo := c.convertPodToPodInfo(&pod)
		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

func (c *Client) GetPod(ctx context.Context, namespace, name string) (*PodInfo, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		c.logger.Error("Error getting pod", "namespace", namespace, "name", name, "error", err)
		return nil, fmt.Errorf("%w: %s/%s", ErrGetPod, namespace, name)
	}

	podInfo := c.convertPodToPodInfo(pod)
	return &podInfo, nil
}

func (c *Client) GetServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	services, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing services", "namespace", namespace, "error", err)
		return nil, fmt.Errorf("%w in namespace %s", ErrListServices, namespace)
	}

	var serviceInfos []ServiceInfo
	for _, service := range services.Items {
		serviceInfo := c.convertServiceToServiceInfo(&service)
		serviceInfos = append(serviceInfos, serviceInfo)
	}

	return serviceInfos, nil
}

func (c *Client) GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error) {
	services, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing services from all namespaces", "error", err)
		return nil, ErrListServices
	}

	var serviceInfos []ServiceInfo
	for _, service := range services.Items {
		serviceInfo := c.convertServiceToServiceInfo(&service)
		serviceInfos = append(serviceInfos, serviceInfo)
	}

	return serviceInfos, nil
}

func (c *Client) GetEvents(ctx context.Context, namespace string) ([]EventInfo, error) {
	events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing events", "namespace", namespace, "error", err)
		return nil, fmt.Errorf("%w in namespace %s", ErrListEvents, namespace)
	}

	var eventInfos []EventInfo
	for _, event := range events.Items {
		eventInfo := c.convertEventToEventInfo(&event)
		eventInfos = append(eventInfos, eventInfo)
	}

	return eventInfos, nil
}

func (c *Client) GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error) {
	events, err := c.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing events from all namespaces", "error", err)
		return nil, ErrListEvents
	}

	var eventInfos []EventInfo
	for _, event := range events.Items {
		eventInfo := c.convertEventToEventInfo(&event)
		eventInfos = append(eventInfos, eventInfo)
	}

	return eventInfos, nil
}

func (c *Client) GetNamespaces(ctx context.Context) ([]string, error) {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing namespaces", "error", err)
		return nil, ErrListNamespaces
	}

	var names []string
	for _, ns := range namespaces.Items {
		names = append(names, ns.Name)
	}

	return names, nil
}

func (c *Client) GetResourceQuotas(ctx context.Context, namespace string) ([]ResourceQuotaInfo, error) {
	quotas, err := c.clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Error("Error listing resource quotas", "namespace", namespace, "error", err)
		return nil, fmt.Errorf("%w in namespace %s", ErrListResourceQuotas, namespace)
	}

	var quotaInfos []ResourceQuotaInfo
	for _, quota := range quotas.Items {
		quotaInfo := c.convertResourceQuotaToResourceQuotaInfo(&quota)
		quotaInfos = append(quotaInfos, quotaInfo)
	}

	return quotaInfos, nil
}

func (c *Client) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		c.logger.Error("Error getting server version", "error", err)
		return nil, ErrGetServerVersion
	}

	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	pods, err := c.GetPodsAllNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	namespaces, err := c.GetNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	return &ClusterInfo{
		Version:    version.GitVersion,
		NodeCount:  len(nodes),
		PodCount:   len(pods),
		Namespaces: namespaces,
		APIVersion: version.Major + "." + version.Minor,
		ServerURL:  c.config.Host,
	}, nil
}

func (c *Client) GetResource(ctx context.Context, namespace, resource string) ([]runtime.Object, error) {
	return nil, ErrNotImplemented
}

func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		c.logger.Error("Health check failed", "error", err)
		return ErrHealthCheckFailed
	}
	return nil
}

func (c *Client) convertNodeToNodeInfo(node *corev1.Node) NodeInfo {
	var conditions []NodeCondition
	for _, condition := range node.Status.Conditions {
		conditions = append(conditions, NodeCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		})
	}

	capacity := make(map[string]string)
	for k, v := range node.Status.Capacity {
		capacity[string(k)] = v.String()
	}

	allocatable := make(map[string]string)
	for k, v := range node.Status.Allocatable {
		allocatable[string(k)] = v.String()
	}

	status := "Unknown"
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
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
		Allocatable: allocatable,
		Age:         time.Since(node.CreationTimestamp.Time),
		Labels:      node.Labels,
		Annotations: node.Annotations,
	}
}

func (c *Client) convertPodToPodInfo(pod *corev1.Pod) PodInfo {
	var containers []ContainerInfo
	restartCount := 0

	for i, container := range pod.Spec.Containers {
		containerInfo := ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		}

		if i < len(pod.Status.ContainerStatuses) {
			status := pod.Status.ContainerStatuses[i]
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
			restartCount += int(status.RestartCount)

			if status.State.Running != nil {
				containerInfo.State = "Running"
			} else if status.State.Waiting != nil {
				containerInfo.State = "Waiting"
			} else if status.State.Terminated != nil {
				containerInfo.State = "Terminated"
			}
		}

		containers = append(containers, containerInfo)
	}

	return PodInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		Status:       string(pod.Status.Phase),
		Phase:        pod.Status.Phase,
		NodeName:     pod.Spec.NodeName,
		Age:          time.Since(pod.CreationTimestamp.Time),
		Labels:       pod.Labels,
		Annotations:  pod.Annotations,
		Containers:   containers,
		RestartCount: restartCount,
	}
}

func (c *Client) convertServiceToServiceInfo(service *corev1.Service) ServiceInfo {
	return ServiceInfo{
		Name:        service.Name,
		Namespace:   service.Namespace,
		Type:        service.Spec.Type,
		ClusterIP:   service.Spec.ClusterIP,
		ExternalIPs: service.Spec.ExternalIPs,
		Ports:       service.Spec.Ports,
		Selector:    service.Spec.Selector,
		Age:         time.Since(service.CreationTimestamp.Time),
	}
}

func (c *Client) convertEventToEventInfo(event *corev1.Event) EventInfo {
	return EventInfo{
		Type:           event.Type,
		Reason:         event.Reason,
		ObjectKind:     event.InvolvedObject.Kind,
		ObjectName:     event.InvolvedObject.Name,
		Message:        event.Message,
		FirstTimestamp: event.FirstTimestamp.Time,
		LastTimestamp:  event.LastTimestamp.Time,
		Count:          event.Count,
		Namespace:      event.Namespace,
	}
}

func (c *Client) convertResourceQuotaToResourceQuotaInfo(quota *corev1.ResourceQuota) ResourceQuotaInfo {
	hard := make(map[string]string)
	for k, v := range quota.Status.Hard {
		hard[string(k)] = v.String()
	}

	used := make(map[string]string)
	for k, v := range quota.Status.Used {
		used[string(k)] = v.String()
	}

	return ResourceQuotaInfo{
		Name:      quota.Name,
		Namespace: quota.Namespace,
		Hard:      hard,
		Used:      used,
	}
}
