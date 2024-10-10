package providers

import (
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"time"
)

type ScenarioTag struct {
	Name         string    `json:"name"`
	Digest       string    `json:"digest"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

type ScenarioDetail struct {
	ScenarioTag
	fields []typing.Field
}
