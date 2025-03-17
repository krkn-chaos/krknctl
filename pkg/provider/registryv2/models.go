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

/*
{
  "id" : "4f3b435feb9336170b53414b0332e5ee35e41a0f20c65a31baf48321ef17ba33",
  "parent" : "daad2e9ade2243b50ba9af5b1e636ff343869b06189cfcfe0ecc28cca57e55e6",
  "created" : "2024-09-18T21:29:42.817435026Z",
  "throwaway" : true,
  "container_config" : {
    "Cmd" : [ "/bin/sh -c #(nop) LABEL com.redhat.license_terms=\"https://www.redhat.com/en/about/red-hat-end-user-license-agreements#UBI\"" ]
  },
  "Size" : 32
}
*/

/*
  "tag" : "dummy-scenario",
  "name" : "rh_ee_tsebasti/krkn-hub-private",
  "architecture" : "amd64",
  "schemaVersion" : 1,
  "history" : [ {
*/
