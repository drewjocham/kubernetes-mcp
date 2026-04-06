package widgets

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type GitStatusWidget struct {
	repoPath string

	mu        sync.RWMutex
	branch    string
	dirty     int
	lastErr   error
	lastAt    time.Time
	aheadInfo string
}

func NewGitStatusWidget(repoPath string) *GitStatusWidget {
	return &GitStatusWidget{repoPath: repoPath}
}

func (w *GitStatusWidget) ID() string {
	return "git-status"
}

func (w *GitStatusWidget) Title() string {
	return "Git Status"
}

func (w *GitStatusWidget) Refresh(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "--no-pager", "-C", w.repoPath, "status", "--porcelain", "--branch")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastAt = time.Now().UTC()
	if err != nil {
		w.lastErr = fmt.Errorf("git status failed: %w", err)
		return w.lastErr
	}

	branch, aheadInfo, dirty := parseGitStatus(stdout.String())
	w.branch = branch
	w.aheadInfo = aheadInfo
	w.dirty = dirty
	w.lastErr = nil
	return nil
}

func (w *GitStatusWidget) View() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.lastErr != nil {
		return fmt.Sprintf("error: %v", w.lastErr)
	}

	branch := w.branch
	if branch == "" {
		branch = "unknown"
	}
	state := "clean"
	if w.dirty > 0 {
		state = fmt.Sprintf("%d changed", w.dirty)
	}

	lines := []string{
		fmt.Sprintf("branch: %s", branch),
		fmt.Sprintf("state: %s", state),
	}
	if w.aheadInfo != "" {
		lines = append(lines, fmt.Sprintf("sync: %s", w.aheadInfo))
	}
	if !w.lastAt.IsZero() {
		lines = append(lines, fmt.Sprintf("updated: %s", w.lastAt.Format("15:04:05")))
	}
	return strings.Join(lines, "\n")
}

func parseGitStatus(output string) (branch string, aheadInfo string, dirty int) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(line, "##") {
			head := strings.TrimSpace(strings.TrimPrefix(line, "##"))
			if idx := strings.Index(head, "..."); idx >= 0 {
				branch = strings.TrimSpace(head[:idx])
				rest := strings.TrimSpace(head[idx+3:])
				if lb := strings.Index(rest, "["); lb >= 0 && strings.HasSuffix(rest, "]") {
					aheadInfo = strings.TrimSuffix(rest[lb+1:], "]")
				}
			} else {
				branch = head
			}
			continue
		}
		dirty++
	}
	return branch, aheadInfo, dirty
}
