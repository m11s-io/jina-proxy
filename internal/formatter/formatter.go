package formatter

import (
	"fmt"
	"github.com/m11s-io/jina-proxy/internal/qwen"
)

type Formatter interface {
	Query(string, string) string
	Document(string) string
}
type Qwen3 struct{}

func (Qwen3) Query(i, q string) string { return qwen.FormatQuery(i, q) }
func (Qwen3) Document(d string) string { return qwen.FormatDocument(d) }
func New(name string) (Formatter, error) {
	if name != "qwen3" {
		return nil, fmt.Errorf("unknown model formatter %q", name)
	}
	return Qwen3{}, nil
}
