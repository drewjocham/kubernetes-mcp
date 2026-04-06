package markdown

type PlainStrategy struct{}

func NewPlainStrategy() *PlainStrategy {
	return &PlainStrategy{}
}

func (p *PlainStrategy) Render(raw []byte) (string, error) {
	return string(raw), nil
}
