package blocks

import (
	"time"

	"github.com/google/uuid"

	"kube-watcher/terminal/internal/domain"
)

type Manager struct {
	blocks   []*domain.Block
	activeID string
}

const MaxLiveBlocks = 50

func (m *Manager) MarkLastBlockExit(exitCode int, hasError bool) {
	if len(m.blocks) == 0 {
		return
	}
	last := m.blocks[len(m.blocks)-1]
	last.ExitCode = exitCode
	last.HasError = hasError
	if last.CompletedAt.IsZero() {
		last.CompletedAt = time.Now().UTC()
	}
	if !last.StartedAt.IsZero() {
		last.Duration = last.CompletedAt.Sub(last.StartedAt)
	}
}

func NewManager() *Manager {
	now := time.Now().UTC()
	initial := &domain.Block{
		ID:          uuid.NewString(),
		ContentType: domain.ContentTypePlainText,
		RenderMode:  domain.RenderModeAuto,
		Active:      true,
		Timestamp:   now,
		StartedAt:   now,
	}

	return &Manager{
		blocks:   []*domain.Block{initial},
		activeID: initial.ID,
	}
}

func (m *Manager) Blocks() []*domain.Block {
	return m.blocks
}

func (m *Manager) AppendToActive(data []byte) {
	active := m.active()
	if active == nil {
		return
	}
	active.RawOutput = append(active.RawOutput, data...)
}

func (m *Manager) SetActiveCommand(command string) {
	active := m.active()
	if active == nil {
		return
	}
	active.Command = command
}

func (m *Manager) SetActiveContentType(contentType string) {
	active := m.active()
	if active == nil {
		return
	}
	active.ContentType = contentType
}
func (m *Manager) SetActiveCWD(cwd string) {
	active := m.active()
	if active == nil {
		return
	}
	active.CWD = cwd
}

func (m *Manager) SealAndNew() {
	now := time.Now().UTC()
	for _, block := range m.blocks {
		if !block.Active {
			continue
		}
		block.Active = false
		if block.CompletedAt.IsZero() {
			block.CompletedAt = now
		}
		if !block.StartedAt.IsZero() {
			block.Duration = block.CompletedAt.Sub(block.StartedAt)
		}
	}

	next := &domain.Block{
		ID:          uuid.NewString(),
		ContentType: domain.ContentTypePlainText,
		RenderMode:  domain.RenderModeAuto,
		Active:      true,
		Timestamp:   now,
		StartedAt:   now,
	}
	m.blocks = append(m.blocks, next)
	m.activeID = next.ID
	m.pruneOldBlocks()
}

func (m *Manager) SetRenderBounds(blockID string, y, height int) {
	for _, block := range m.blocks {
		if block.ID == blockID {
			block.RenderY = y
			block.Height = height
			return
		}
	}
}

func (m *Manager) FindByRenderY(y int) *domain.Block {
	for _, block := range m.blocks {
		if block.Height <= 0 {
			continue
		}
		if y >= block.RenderY && y < block.RenderY+block.Height {
			return block
		}
	}
	return nil
}

func (m *Manager) FindByID(blockID string) *domain.Block {
	for _, block := range m.blocks {
		if block.ID == blockID {
			return block
		}
	}
	return nil
}

func (m *Manager) ToggleBlockRenderMode(blockID string) string {
	block := m.FindByID(blockID)
	if block == nil {
		return ""
	}
	switch block.RenderMode {
	case domain.RenderModeAuto:
		block.RenderMode = domain.RenderModeMarkdown
	case domain.RenderModeMarkdown:
		block.RenderMode = domain.RenderModePlain
	default:
		block.RenderMode = domain.RenderModeAuto
	}
	return block.RenderMode
}

func (m *Manager) pruneOldBlocks() {
	if len(m.blocks) <= MaxLiveBlocks {
		return
	}
	pruneCount := len(m.blocks) - MaxLiveBlocks
	m.blocks = m.blocks[pruneCount:]
	if m.active() == nil && len(m.blocks) > 0 {
		m.activeID = m.blocks[len(m.blocks)-1].ID
	}
}

func (m *Manager) active() *domain.Block {
	for _, block := range m.blocks {
		if block.ID == m.activeID {
			return block
		}
	}
	return nil
}
