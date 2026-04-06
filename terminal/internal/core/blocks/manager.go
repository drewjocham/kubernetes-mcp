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

func (m *Manager) MarkLastBlockExit(exitCode int, hasError bool) {
	if len(m.blocks) == 0 {
		return
	}
	last := m.blocks[len(m.blocks)-1]
	last.ExitCode = exitCode
	last.HasError = hasError
}

func NewManager() *Manager {
	initial := &domain.Block{
		ID:          uuid.NewString(),
		ContentType: domain.ContentTypePlainText,
		Active:      true,
		Timestamp:   time.Now().UTC(),
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

func (m *Manager) SealAndNew() {
	for _, block := range m.blocks {
		block.Active = false
	}

	next := &domain.Block{
		ID:          uuid.NewString(),
		ContentType: domain.ContentTypePlainText,
		Active:      true,
		Timestamp:   time.Now().UTC(),
	}
	m.blocks = append(m.blocks, next)
	m.activeID = next.ID
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

func (m *Manager) active() *domain.Block {
	for _, block := range m.blocks {
		if block.ID == m.activeID {
			return block
		}
	}
	return nil
}
