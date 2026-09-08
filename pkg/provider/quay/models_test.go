package quay

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestList_GetKrknctlManifest_MatchesArchitecture(t *testing.T) {
	list := ManifestList{Manifests: []ManifestEntry{
		{Digest: "sha256:other", Platform: Platform{Architecture: "other-arch"}},
		{Digest: "sha256:matching", Platform: Platform{Architecture: runtime.GOARCH}},
	}}

	selected := list.GetKrknctlManifest()
	require.NotNil(t, selected)
	assert.Equal(t, "sha256:matching", selected.Digest)
}

func TestManifestList_GetKrknctlManifest_SkipsWindows(t *testing.T) {
	list := ManifestList{Manifests: []ManifestEntry{
		{Digest: "sha256:windows", Platform: Platform{OS: "windows", Architecture: runtime.GOARCH}},
		{Digest: "sha256:linux", Platform: Platform{OS: "linux", Architecture: "other-arch"}},
	}}

	selected := list.GetKrknctlManifest()
	require.NotNil(t, selected)
	assert.Equal(t, "sha256:linux", selected.Digest)
}

func TestManifestList_GetKrknctlManifest_FallsBackToFirstUsable(t *testing.T) {
	list := ManifestList{Manifests: []ManifestEntry{
		{Digest: "", Platform: Platform{Architecture: runtime.GOARCH}},
		{Digest: "sha256:first-usable", Platform: Platform{Architecture: "other-arch"}},
		{Digest: "sha256:second-usable", Platform: Platform{Architecture: "another-arch"}},
	}}

	selected := list.GetKrknctlManifest()
	require.NotNil(t, selected)
	assert.Equal(t, "sha256:first-usable", selected.Digest)
}

func TestManifestList_GetKrknctlManifest_Empty(t *testing.T) {
	assert.Nil(t, (ManifestList{}).GetKrknctlManifest())
}

func TestManifestImageSize_UsesAggregateSize(t *testing.T) {
	size := manifestImageSize(Manifest{LayerCompressedSize: "5242880"})
	require.NotNil(t, size)
	assert.Equal(t, int64(5242880), *size)
}

func TestManifestImageSize_SumsLayersWhenAggregateMissing(t *testing.T) {
	size := manifestImageSize(Manifest{Layers: []Layer{
		{CompressedSize: 100},
		{CompressedSize: 250},
	}})
	require.NotNil(t, size)
	assert.Equal(t, int64(350), *size)
}

func TestManifestImageSize_UnknownForIncompleteLayers(t *testing.T) {
	assert.Nil(t, manifestImageSize(Manifest{Layers: []Layer{
		{CompressedSize: 100},
		{CompressedSize: 0},
	}}))
	assert.Nil(t, manifestImageSize(Manifest{LayerCompressedSize: "not-a-number"}))
	assert.Nil(t, manifestImageSize(Manifest{}))
}

func TestPreserveKnownImageSizeWhenManifestMetadataIsIncomplete(t *testing.T) {
	existing := int64(6179)
	assert.Equal(t, int64(6179), *preserveKnownImageSize(&existing, Manifest{}))

	replacement := preserveKnownImageSize(&existing, Manifest{LayerCompressedSize: "8192"})
	assert.Equal(t, int64(8192), *replacement)
}
