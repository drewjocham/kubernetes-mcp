package widgets

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
)

type SystemStatsWidget struct {
	mu       sync.RWMutex
	cpuCount int
	goCount  int
	memUsed  uint64
	memTotal uint64
	lastErr  error
	lastAt   time.Time
}

func NewSystemStatsWidget() *SystemStatsWidget {
	return &SystemStatsWidget{}
}

func (w *SystemStatsWidget) ID() string {
	return "system-stats"
}

func (w *SystemStatsWidget) Title() string {
	return "System Stats"
}

func (w *SystemStatsWidget) Refresh(_ context.Context) error {
	vm, err := mem.VirtualMemory()

	w.mu.Lock()
	defer w.mu.Unlock()

	w.cpuCount = runtime.NumCPU()
	w.goCount = runtime.NumGoroutine()
	w.lastAt = time.Now().UTC()
	w.lastErr = err
	if err == nil {
		w.memUsed = vm.Used
		w.memTotal = vm.Total
	}

	return err
}

func (w *SystemStatsWidget) View() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.lastErr != nil {
		return fmt.Sprintf("error: %v", w.lastErr)
	}

	memLine := "mem: n/a"
	if w.memTotal > 0 {
		memLine = fmt.Sprintf("mem: %s / %s", formatBytes(w.memUsed), formatBytes(w.memTotal))
	}

	lines := []string{
		fmt.Sprintf("cpus: %d", w.cpuCount),
		fmt.Sprintf("goroutines: %d", w.goCount),
		memLine,
	}
	if !w.lastAt.IsZero() {
		lines = append(lines, fmt.Sprintf("updated: %s", w.lastAt.Format("15:04:05")))
	}
	return strings.Join(lines, "\n")
}

func formatBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := "KMGTPE"[exp]
	return fmt.Sprintf("%.1f %ciB", float64(v)/float64(div), suffix)
}
