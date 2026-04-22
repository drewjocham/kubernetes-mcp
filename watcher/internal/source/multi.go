package source

import (
	"context"
	"log/slog"
	"sync"

	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/pipeline"
)

type MultiSource struct {
	logger  *slog.Logger
	sources []pipeline.Source
}

func NewMultiSource(logger *slog.Logger, sources ...pipeline.Source) *MultiSource {
	return &MultiSource{
		logger:  logger,
		sources: sources,
	}
}

func (m *MultiSource) Run(ctx context.Context, out chan<- events.ResourceEvent) {
	var wg sync.WaitGroup
	for _, src := range m.sources {
		wg.Add(1)
		go func(s pipeline.Source) {
			defer wg.Done()
			s.Run(ctx, out)
		}(src)
	}
	wg.Wait()
}
