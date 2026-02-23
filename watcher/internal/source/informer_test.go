package source

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"kube-watcher/watcher/internal/events"
)

func TestInformerSource_Run(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	testCases := []struct {
		name          string
		initialObject *v1.Pod
		updateObject  *v1.Pod
		deleteObject  *v1.Pod
		expectedCount int
	}{
		{
			name: "add, update, and delete events",
			initialObject: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default", ResourceVersion: "1"},
			},
			updateObject: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default", ResourceVersion: "2"},
			},
			deleteObject: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default", ResourceVersion: "3"},
			},
			expectedCount: 3,
		},
		{
			name: "add event only",
			initialObject: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod-2", Namespace: "default", ResourceVersion: "1"},
			},
			expectedCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			if tc.initialObject != nil {
				client = fake.NewSimpleClientset(tc.initialObject)
			}

			source := NewInformerSource(client, logger, 1*time.Minute)
			out := make(chan events.ResourceEvent, 10)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go source.Run(ctx, out)

			// Allow informers to sync
			time.Sleep(100 * time.Millisecond)

			if tc.updateObject != nil {
				_, err := client.CoreV1().Pods("default").Update(ctx, tc.updateObject, metav1.UpdateOptions{})
				assert.NoError(t, err)
			}
			if tc.deleteObject != nil {
				err := client.CoreV1().Pods("default").Delete(ctx, tc.deleteObject.Name, metav1.DeleteOptions{})
				assert.NoError(t, err)
			}

			// Wait for events to be processed
			time.Sleep(100 * time.Millisecond)
			cancel() // Stop the source

			close(out)
			var receivedEvents []events.ResourceEvent
			for evt := range out {
				receivedEvents = append(receivedEvents, evt)
			}

			// The fake client can sometimes generate extra events, so we check for at least the expected number.
			assert.GreaterOrEqual(t, len(receivedEvents), tc.expectedCount, "did not receive expected number of events")

			if len(receivedEvents) > 0 {
				firstEvent := receivedEvents[0]
				assert.Equal(t, "Pod", firstEvent.Kind)
				assert.Equal(t, tc.initialObject.Name, firstEvent.Name)
				assert.Equal(t, tc.initialObject.Namespace, firstEvent.Namespace)
			}
		})
	}
}

func TestObjectMeta(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}}

	testCases := []struct {
		name     string
		obj      interface{}
		wantName string
		wantOk   bool
	}{
		{
			name:     "v1.Pod object",
			obj:      pod,
			wantName: "test-pod",
			wantOk:   true,
		},
		{
			name: "DeletedFinalStateUnknown object",
			obj: cache.DeletedFinalStateUnknown{
				Key: "pod/test-pod",
				Obj: pod,
			},
			wantName: "test-pod",
			wantOk:   true,
		},
		{
			name:   "nil object",
			obj:    nil,
			wantOk: false,
		},
		{
			name:   "unsupported type",
			obj:    "a string",
			wantOk: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, meta := objectMeta(tc.obj)
			if tc.wantOk {
				assert.NotNil(t, meta)
				assert.Equal(t, tc.wantName, meta.GetName())
			} else {
				assert.Nil(t, meta)
			}
		})
	}
}
