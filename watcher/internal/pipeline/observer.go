package pipeline

import "kube-watcher/watcher/internal/events"

type Observer interface {
	Observe(events.ResourceEvent)
}
