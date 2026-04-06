package markdown

import (
	"fmt"

	"github.com/charmbracelet/glamour"
)

type GlamourStrategy struct {
	renderer *glamour.TermRenderer
}

func NewGlamourStrategy(width int) (*GlamourStrategy, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, fmt.Errorf("create glamour renderer: %w", err)
	}
	return &GlamourStrategy{renderer: renderer}, nil
}

func (g *GlamourStrategy) Render(raw []byte) (string, error) {
	out, err := g.renderer.Render(string(raw))
	if err != nil {
		return "", fmt.Errorf("render markdown block: %w", err)
	}
	return out, nil
}
