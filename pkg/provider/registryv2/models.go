package registryv2

type TagsV2 struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
