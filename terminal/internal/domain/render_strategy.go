package domain

type RenderStrategy interface {
	Render(raw []byte) (string, error)
}
