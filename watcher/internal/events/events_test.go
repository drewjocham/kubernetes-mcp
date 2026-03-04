package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceEvent_Key(t *testing.T) {
	testCases := []struct {
		name    string
		event   ResourceEvent
		wantKey string
	}{
		{
			name: "pod event",
			event: ResourceEvent{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "my-pod",
			},
			wantKey: "pod/default/my-pod",
		},
		{
			name: "node event",
			event: ResourceEvent{
				Kind: "Node",
				Name: "worker-1",
			},
			wantKey: "node//worker-1",
		},
		{
			name: "event with mixed case kind",
			event: ResourceEvent{
				Kind:      "Deployment",
				Namespace: "kube-system",
				Name:      "coredns",
			},
			wantKey: "deployment/kube-system/coredns",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.event.Key()
			assert.Equal(t, tc.wantKey, key)
		})
	}
}

func TestResourceEvent_ResourceVersionValue(t *testing.T) {
	testCases := []struct {
		name                string
		event               ResourceEvent
		wantResourceVersion string
	}{
		{
			name: "with resource version",
			event: ResourceEvent{
				ResourceVersion: "12345",
			},
			wantResourceVersion: "12345",
		},
		{
			name: "without resource version",
			event: ResourceEvent{
				ResourceVersion: "",
			},
			wantResourceVersion: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rv := tc.event.ResourceVersionValue()
			assert.Equal(t, tc.wantResourceVersion, rv)
		})
	}
}
