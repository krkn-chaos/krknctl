package registryv2

import (
	"fmt"
	"strings"
)

type TagsV2 struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type ManifestV2 struct {
	Tag           string              `json:"tag"`
	Name          string              `json:"name"`
	Architecture  string              `json:"architecture"`
	SchemaVersion int                 `json:"schemaVersion"`
	RawLayers     []map[string]string `json:"history"`
	Layers        []LayerV1Compat
	Config        ManifestDescriptor `json:"config"`
}

// ManifestDescriptor is used by Docker Schema 2 and OCI manifests to point
// at the image configuration blob.
type ManifestDescriptor struct {
	Digest string `json:"digest"`
}

type ImageConfig struct {
	Config ImageConfigData `json:"config"`
}

type ImageConfigData struct {
	Labels map[string]string `json:"Labels"`
}

func imageConfigLabelsToCommands(labels map[string]string) []string {
	commands := make([]string, 0, len(labels))
	for name, value := range labels {
		label := strings.TrimSuffix(name, "=")
		if strings.HasSuffix(name, "input_fields") || strings.HasSuffix(name, "input_fields.global") {
			commands = append(commands, fmt.Sprintf("LABEL %s='%s'", label, value))
			continue
		}
		commands = append(commands, fmt.Sprintf("LABEL %s=%s", label, value))
	}
	return commands
}

type LayerV1Compat struct {
	ID              string          `json:"id"`
	Parent          string          `json:"parent"`
	Created         string          `json:"created"`
	Throwaway       bool            `json:"throwaway"`
	ContainerConfig containerConfig `json:"container_config"`
	Size            int64           `json:"size"`
}

// containerConfig captures only the field krknctl needs (the build Cmd, which
// carries the LABEL instructions) from a Docker schema1 v1Compatibility
// container_config. It deliberately ignores every other key.
//
// The previous typing, map[string][]string, could never decode a real manifest:
// a container_config also contains string- and bool-valued keys (Hostname,
// AttachStdin, ...), and json.Unmarshal aborts the whole object on the first
// value whose type is not []string. That error made getScenarioDetail skip
// every layer, so no LABEL was ever found and scenario detail lookups failed for
// real registry images.
type containerConfig struct {
	Cmd []string `json:"Cmd"`
}

func (l LayerV1Compat) GetCommands() []string {
	return l.ContainerConfig.Cmd
}
