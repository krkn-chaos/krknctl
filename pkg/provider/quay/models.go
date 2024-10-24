package quay

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TagPage struct {
	Tags          []Tag `json:"tags"`
	Page          int   `json:"page"`
	HasAdditional bool  `json:"has_additional"`
}

type Tag struct {
	Name           string    `json:"name"`
	Reversion      bool      `json:"reversion"`
	StartTimeStamp int64     `json:"start_ts"`
	ManifestDigest string    `json:"manifest_digest"`
	IsManifestList bool      `json:"is_manifest_list"`
	Size           int64     `json:"size"`
	LastModified   time.Time `json:"last_modified"`
}

type tagAlias Tag

func (q *Tag) UnmarshalJSON(bytes []byte) error {
	aux := &struct {
		LastModified string `json:"last_modified"`
		*tagAlias
	}{
		tagAlias: (*tagAlias)(q),
	}
	if err := json.Unmarshal(bytes, &aux); err != nil {
		return err
	}

	layout := "Mon, 02 Jan 2006 15:04:05 -0700"
	parsedTime, err := time.Parse(layout, aux.LastModified)
	if err != nil {
		return err
	}

	q.LastModified = parsedTime

	return nil
}

type Manifest struct {
	Digest              string  `json:"digest"`
	IsManifestList      bool    `json:"is_manifest_list"`
	ManifestData        string  `json:"manifest_data"`
	ConfigMediaType     string  `json:"config_media_type"`
	LayerCompressedSize string  `json:"layer_compressed_size"`
	Layers              []Layer `json:"layers"`
}

func (m *Manifest) GetKrknctlLabel(label string) *string {
	for _, v := range m.Layers {
		for _, c := range v.Command {
			if strings.Contains(c, fmt.Sprintf("LABEL %s", label)) {
				return &c
			}
		}
	}
	return nil
}

type Layer struct {
	Index           int64     `json:"index"`
	CompressedSize  int64     `json:"compressed_size"`
	IsRemote        bool      `json:"is_remote"`
	Command         []string  `json:"command"`
	Comment         string    `json:"comment"`
	Author          string    `json:"author"`
	BlobDigest      string    `json:"blob_digest"`
	CreatedDateTime time.Time `json:"created_datetime"`
}

type layerAlias Layer

func (l *Layer) UnmarshalJSON(bytes []byte) error {
	aux := &struct {
		CreatedDateTime string `json:"created_datetime"`
		*layerAlias
	}{
		layerAlias: (*layerAlias)(l),
	}
	if err := json.Unmarshal(bytes, &aux); err != nil {
		return err
	}

	layout := "Mon, 02 Jan 2006 15:04:05 -0700"
	parsedTime, err := time.Parse(layout, aux.CreatedDateTime)
	if err != nil {
		return err
	}

	l.CreatedDateTime = parsedTime

	return nil
}
