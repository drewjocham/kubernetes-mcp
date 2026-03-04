package watch

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestPodEvaluatorHandlesNilWaiting(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "api",
					RestartCount: 6,
					State: corev1.ContainerState{
						Waiting: nil,
					},
				},
			},
		},
	}

	alert := PodEvaluator(pod)
	if alert == nil {
		t.Fatalf("expected alert when restart count high")
	}
	if alert.Reason != "restart_threshold" {
		t.Fatalf("expected default reason, got %s", alert.Reason)
	}
}

func TestPodEvaluatorCrashLoopReason(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "api",
					RestartCount: 1,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}

	alert := PodEvaluator(pod)
	if alert == nil {
		t.Fatalf("expected alert for crash loop")
	}
	if alert.Reason != "CrashLoopBackOff" {
		t.Fatalf("expected crash reason, got %s", alert.Reason)
	}
}
