package events

import (
	"fmt"
	"strings"
)

type ResourceEvent struct {
	Kind            string
	Namespace       string
	Name            string
	ResourceVersion string
	Object          map[string]interface{}
	Raw             interface{}
}

func (e ResourceEvent) Key() string {
	return fmt.Sprintf("%s/%s/%s", strings.ToLower(e.Kind), e.Namespace, e.Name)
}

func (e ResourceEvent) ResourceVersionValue() string {
	return e.ResourceVersion
}
