package kube

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"kube-watcher/pkg/audit"
)

// AuditClient wraps a kube.Client with audit logging and RBAC validation
type AuditClient struct {
	client      ClientInterface
	auditLogger audit.Logger
	logger      *slog.Logger
	// serviceAccount is the name of the service account used for operations
	serviceAccount string
	// performRBACValidation enables pre-flight RBAC checks (optional)
	performRBACValidation bool
}

// AuditClientOption configures the AuditClient
type AuditClientOption func(*AuditClient)

// WithRBACValidation enables pre-flight RBAC validation
func WithRBACValidation(enabled bool) AuditClientOption {
	return func(ac *AuditClient) {
		ac.performRBACValidation = enabled
	}
}

// WithServiceAccount sets the service account name for audit logs
func WithServiceAccount(name string) AuditClientOption {
	return func(ac *AuditClient) {
		ac.serviceAccount = name
	}
}

const (
	envAuditRBACEnabled = "KUBE_WATCHER_AUDIT_RBAC_ENABLED"
	envServiceAccount   = "KUBE_WATCHER_SERVICE_ACCOUNT"
)

// AuditOptionsFromEnv reads environment variables and returns applicable AuditClientOptions
func AuditOptionsFromEnv() []AuditClientOption {
	var opts []AuditClientOption

	// Check RBAC validation
	if rbacEnabled := os.Getenv(envAuditRBACEnabled); strings.ToLower(rbacEnabled) == "true" {
		opts = append(opts, WithRBACValidation(true))
	}

	// Check service account
	if sa := os.Getenv(envServiceAccount); sa != "" {
		opts = append(opts, WithServiceAccount(sa))
	}

	return opts
}

// NewAuditClient creates a new AuditClient wrapping the provided client
func NewAuditClient(client ClientInterface, auditLogger audit.Logger, logger *slog.Logger, opts ...AuditClientOption) *AuditClient {
	ac := &AuditClient{
		client:                client,
		auditLogger:           auditLogger,
		logger:                logger,
		serviceAccount:        "unknown", // Default if not specified
		performRBACValidation: false,     // Off by default to avoid extra API calls
	}

	for _, opt := range opts {
		opt(ac)
	}

	return ac
}

// checkRBAC performs a SubjectAccessReview to check permissions
func (ac *AuditClient) checkRBAC(ctx context.Context, resource, namespace, name string, verb string) (bool, string, error) {
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Resource:  resource,
				Name:      name,
			},
			User: ac.serviceAccount,
		},
	}

	result, err := ac.client.GetRawInterface().AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Sprintf("SAR request failed: %v", err), err
	}

	if result.Status.Allowed {
		return true, "", nil
	}

	reason := "denied"
	if result.Status.Reason != "" {
		reason = result.Status.Reason
	}
	return false, reason, nil
}

// logOperation logs an audit event for a Kubernetes operation
func (ac *AuditClient) logOperation(ctx context.Context, eventType audit.EventType, action audit.Action, resource, namespace, name string, outcome audit.Outcome, reason string) {
	ac.auditLogger.Log(ctx, audit.AuditEvent{
		Timestamp:      time.Now(),
		EventType:      eventType,
		Action:         action,
		Resource:       resource,
		Namespace:      namespace,
		Name:           name,
		ServiceAccount: ac.serviceAccount,
		Outcome:        outcome,
		Reason:         reason,
	})
}

// logAndError logs an audit event and returns the error
func (ac *AuditClient) logAndError(ctx context.Context, eventType audit.EventType, action audit.Action, resource, namespace, name string, err error) error {
	outcome := audit.OutcomeError
	reason := err.Error()

	// Check if it's an authorization error
	if errors.IsForbidden(err) {
		eventType = audit.EventTypeAuthz
		outcome = audit.OutcomeDenied
		reason = "forbidden"
	} else if errors.IsUnauthorized(err) {
		eventType = audit.EventTypeAuthn
		outcome = audit.OutcomeDenied
		reason = "unauthorized"
	}

	ac.logOperation(ctx, eventType, action, resource, namespace, name, outcome, reason)
	return err
}

// GetNodes implements ClientInterface.GetNodes with audit logging
func (ac *AuditClient) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	action := audit.ActionList
	resource := "nodes"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, "", "", "list")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, "", "", audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), "", fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	nodes, err := ac.client.GetNodes(ctx)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, "", "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, "", "", audit.OutcomeSuccess, "")
	return nodes, nil
}

// GetNode implements ClientInterface.GetNode with audit logging
func (ac *AuditClient) GetNode(ctx context.Context, nodeName string) (*NodeInfo, error) {
	action := audit.ActionGet
	resource := "nodes"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, "", nodeName, "get")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "name", nodeName)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, "", nodeName, audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), nodeName, fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	node, err := ac.client.GetNode(ctx, nodeName)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, "", nodeName, err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, "", nodeName, audit.OutcomeSuccess, "")
	return node, nil
}

// GetPods implements ClientInterface.GetPods with audit logging
func (ac *AuditClient) GetPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	action := audit.ActionList
	resource := "pods"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, namespace, "", "list")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "namespace", namespace)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, namespace, "", audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), "", fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	pods, err := ac.client.GetPods(ctx, namespace)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, namespace, "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, namespace, "", audit.OutcomeSuccess, "")
	return pods, nil
}

// GetPodsAllNamespaces implements ClientInterface.GetPodsAllNamespaces with audit logging
func (ac *AuditClient) GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error) {
	// This is essentially GetPods with empty namespace
	return ac.GetPods(ctx, "")
}

// GetPod implements ClientInterface.GetPod with audit logging
func (ac *AuditClient) GetPod(ctx context.Context, namespace, name string) (*PodInfo, error) {
	action := audit.ActionGet
	resource := "pods"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, namespace, name, "get")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "namespace", namespace, "name", name)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, namespace, name, audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), name, fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	pod, err := ac.client.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, namespace, name, err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, namespace, name, audit.OutcomeSuccess, "")
	return pod, nil
}

// GetPodLogs implements ClientInterface.GetPodLogs with audit logging
func (ac *AuditClient) GetPodLogs(ctx context.Context, namespace, podName, container string, tailLines, sinceSeconds int64, previous bool) (string, error) {
	action := audit.ActionGet
	resource := "pods/log"

	// Optional RBAC validation - need to check for "get" on pods/log subresource
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, "pods/log", namespace, podName, "get")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "namespace", namespace, "name", podName)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, namespace, podName, audit.OutcomeDenied, reason)
			return "", errors.NewForbidden(corev1.Resource("pods/log"), podName, fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	logs, err := ac.client.GetPodLogs(ctx, namespace, podName, container, tailLines, sinceSeconds, previous)
	if err != nil {
		return "", ac.logAndError(ctx, audit.EventTypeAccess, action, resource, namespace, podName, err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, namespace, podName, audit.OutcomeSuccess, "")
	return logs, nil
}

// GetServices implements ClientInterface.GetServices with audit logging
func (ac *AuditClient) GetServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	action := audit.ActionList
	resource := "services"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, namespace, "", "list")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "namespace", namespace)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, namespace, "", audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), "", fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	services, err := ac.client.GetServices(ctx, namespace)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, namespace, "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, namespace, "", audit.OutcomeSuccess, "")
	return services, nil
}

// GetServicesAllNamespaces implements ClientInterface.GetServicesAllNamespaces with audit logging
func (ac *AuditClient) GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error) {
	return ac.GetServices(ctx, "")
}

// GetEvents implements ClientInterface.GetEvents with audit logging
func (ac *AuditClient) GetEvents(ctx context.Context, namespace string) ([]EventInfo, error) {
	action := audit.ActionList
	resource := "events"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, namespace, "", "list")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "namespace", namespace)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, namespace, "", audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), "", fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	events, err := ac.client.GetEvents(ctx, namespace)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, namespace, "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, namespace, "", audit.OutcomeSuccess, "")
	return events, nil
}

// GetEventsAllNamespaces implements ClientInterface.GetEventsAllNamespaces with audit logging
func (ac *AuditClient) GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error) {
	return ac.GetEvents(ctx, "")
}

// GetNamespaces implements ClientInterface.GetNamespaces with audit logging
func (ac *AuditClient) GetNamespaces(ctx context.Context) ([]string, error) {
	action := audit.ActionList
	resource := "namespaces"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, "", "", "list")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, "", "", audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), "", fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	namespaces, err := ac.client.GetNamespaces(ctx)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, "", "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, "", "", audit.OutcomeSuccess, "")
	return namespaces, nil
}

// GetResourceQuotas implements ClientInterface.GetResourceQuotas with audit logging
func (ac *AuditClient) GetResourceQuotas(ctx context.Context, namespace string) ([]ResourceQuotaInfo, error) {
	action := audit.ActionList
	resource := "resourcequotas"

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, namespace, "", "list")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "namespace", namespace)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, namespace, "", audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), "", fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	quotas, err := ac.client.GetResourceQuotas(ctx, namespace)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, namespace, "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, namespace, "", audit.OutcomeSuccess, "")
	return quotas, nil
}

// GetClusterInfo implements ClientInterface.GetClusterInfo with audit logging
func (ac *AuditClient) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	action := audit.ActionGet
	resource := "cluster"

	// This operation requires permissions for discovery API and basic resources
	// We'll log it but skip RBAC validation as it's complex

	info, err := ac.client.GetClusterInfo(ctx)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, "", "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, "", "", audit.OutcomeSuccess, "")
	return info, nil
}

// GetResource implements ClientInterface.GetResource with audit logging
func (ac *AuditClient) GetResource(ctx context.Context, namespace, resource string) ([]runtime.Object, error) {
	action := audit.ActionList

	// Optional RBAC validation
	if ac.performRBACValidation {
		allowed, reason, err := ac.checkRBAC(ctx, resource, namespace, "", "list")
		if err != nil {
			ac.logger.Warn("RBAC validation failed", "error", err, "resource", resource, "namespace", namespace)
		} else if !allowed {
			ac.logOperation(ctx, audit.EventTypeAuthz, action, resource, namespace, "", audit.OutcomeDenied, reason)
			return nil, errors.NewForbidden(corev1.Resource(resource), "", fmt.Errorf("RBAC validation failed: %s", reason))
		}
	}

	objects, err := ac.client.GetResource(ctx, namespace, resource)
	if err != nil {
		return nil, ac.logAndError(ctx, audit.EventTypeAccess, action, resource, namespace, "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, namespace, "", audit.OutcomeSuccess, "")
	return objects, nil
}

// HealthCheck implements ClientInterface.HealthCheck with audit logging
func (ac *AuditClient) HealthCheck(ctx context.Context) error {
	action := audit.ActionGet
	resource := "health"

	err := ac.client.HealthCheck(ctx)
	if err != nil {
		return ac.logAndError(ctx, audit.EventTypeAccess, action, resource, "", "", err)
	}

	ac.logOperation(ctx, audit.EventTypeAccess, action, resource, "", "", audit.OutcomeSuccess, "")
	return nil
}

// GetRawInterface implements ClientInterface.GetRawInterface
func (ac *AuditClient) GetRawInterface() clientset.Interface {
	return ac.client.GetRawInterface()
}
