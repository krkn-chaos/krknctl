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
	Id              string              `json:"id"`
	Parent          string              `json:"parent"`
	Created         string              `json:"created"`
	Throwaway       bool                `json:"throwaway"`
	ContainerConfig map[string][]string `json:"container_config"`
	Size            int64               `json:"size"`
}

func (l LayerV1Compat) GetCommands() []string {
	if val, ok := l.ContainerConfig["Cmd"]; ok {
		return val
	}
	return []string{}
}
