package registryv2

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
