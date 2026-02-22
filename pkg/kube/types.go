package kube

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
)

type NodeInfo struct {
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Conditions  []NodeCondition   `json:"conditions"`
	Taints      []corev1.Taint    `json:"taints"`
	Capacity    map[string]string `json:"capacity"`
	Allocatable map[string]string `json:"allocatable"`
	Age         time.Duration     `json:"age"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// --- Pod Types ---

type PodInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Status       string            `json:"status"`
	Phase        corev1.PodPhase   `json:"phase"`
	NodeName     string            `json:"nodeName"`
	Age          time.Duration     `json:"age"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Containers   []ContainerInfo   `json:"containers"`
	RestartCount int               `json:"restartCount"`
}

type ContainerInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	State        string `json:"state"`
}

type ServiceInfo struct {
	Name        string               `json:"name"`
	Namespace   string               `json:"namespace"`
	Type        corev1.ServiceType   `json:"type"`
	ClusterIP   string               `json:"clusterIP"`
	ExternalIPs []string             `json:"externalIPs"`
	Ports       []corev1.ServicePort `json:"ports"`
	Selector    map[string]string    `json:"selector"`
	Age         time.Duration        `json:"age"`
}

type EventInfo struct {
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	ObjectName     string    `json:"objectName"`
	ObjectKind     string    `json:"objectKind"`
	Message        string    `json:"message"`
	FirstTimestamp time.Time `json:"firstTimestamp"`
	LastTimestamp  time.Time `json:"lastTimestamp"`
	Count          int32     `json:"count"`
	Namespace      string    `json:"namespace"`
}

type ResourceQuotaInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Hard      map[string]string `json:"hard"`
	Used      map[string]string `json:"used"`
}

type ClusterInfo struct {
	Version    string   `json:"version"`
	NodeCount  int      `json:"nodeCount"`
	PodCount   int      `json:"podCount"`
	Namespaces []string `json:"namespaces"`
	APIVersion string   `json:"apiVersion"`
	ServerURL  string   `json:"serverURL"`
}

type ClientInterface interface {
	// Node Operations
	GetNodes(ctx context.Context) ([]NodeInfo, error)
	GetNode(ctx context.Context, nodeName string) (*NodeInfo, error)

	// Pod Operations
	GetPods(ctx context.Context, namespace string) ([]PodInfo, error)
	GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error)
	GetPod(ctx context.Context, namespace, name string) (*PodInfo, error)

	// Service Operations
	GetServices(ctx context.Context, namespace string) ([]ServiceInfo, error)
	GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error)

	// Event Operations
	GetEvents(ctx context.Context, namespace string) ([]EventInfo, error)
	GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error)

	//  Metadata
	GetNamespaces(ctx context.Context) ([]string, error)
	GetResourceQuotas(ctx context.Context, namespace string) ([]ResourceQuotaInfo, error)
	GetClusterInfo(ctx context.Context) (*ClusterInfo, error)
	GetResource(ctx context.Context, namespace, resource string) ([]runtime.Object, error)
	HealthCheck(ctx context.Context) error
	GetRawInterface() clientset.Interface
}
