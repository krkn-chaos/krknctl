package registryv2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realV1Compatibility is a trimmed-but-representative Docker schema1
// v1Compatibility container_config as it actually appears in a registry
// manifest history entry. The critical property is that it mixes value types:
// Hostname/User are strings, AttachStdin/Tty are booleans, and only Cmd/Env are
// string arrays. The previous ContainerConfig typing (map[string][]string)
// could not decode this — json.Unmarshal aborts on the first non-[]string value
// — which is exactly why getScenarioDetail's `continue` used to drop every
// layer and no LABEL was ever found.
const realV1Compatibility = `{
	"id": "abc123",
	"parent": "def456",
	"created": "2024-01-01T00:00:00Z",
	"container_config": {
		"Hostname": "buildhost",
		"User": "root",
		"AttachStdin": false,
		"Tty": false,
		"Env": ["PATH=/usr/bin"],
		"Cmd": ["/bin/sh", "-c", "LABEL krknctl.title=\"My Scenario\""]
	},
	"throwaway": false,
	"Size": 42
}`

// TestLayerV1Compat_UnmarshalMixedTypes proves the model now decodes a real
// mixed-type container_config without error and extracts the build Cmd. Before
// the fix this unmarshal failed with "cannot unmarshal string into Go struct
// field ... of type []string", so this test would have errored.
func TestLayerV1Compat_UnmarshalMixedTypes(t *testing.T) {
	var layer LayerV1Compat
	err := json.Unmarshal([]byte(realV1Compatibility), &layer)
	require.NoError(t, err, "mixed-type container_config must decode without error")

	assert.Equal(t, "abc123", layer.ID)
	assert.Equal(t, "def456", layer.Parent)
	assert.Equal(t, int64(42), layer.Size)

	cmds := layer.GetCommands()
	require.Len(t, cmds, 3)
	assert.Equal(t, "/bin/sh", cmds[0])
	assert.Contains(t, cmds[2], `krknctl.title="My Scenario"`)
}

// TestLayerV1Compat_NoContainerConfig ensures a layer entry that omits
// container_config (common for throwaway/metadata layers) decodes cleanly and
// yields no commands, rather than panicking on a nil map.
func TestLayerV1Compat_NoContainerConfig(t *testing.T) {
	var layer LayerV1Compat
	err := json.Unmarshal([]byte(`{"id":"x","throwaway":true}`), &layer)
	require.NoError(t, err)
	assert.Empty(t, layer.GetCommands())
}

// TestLayerV1Compat_EmptyCmd ensures a present-but-empty Cmd yields an empty
// slice, matching the ContainerLayer contract expected by GetKrknctlLabel.
func TestLayerV1Compat_EmptyCmd(t *testing.T) {
	var layer LayerV1Compat
	err := json.Unmarshal([]byte(`{"container_config":{"Cmd":[]}}`), &layer)
	require.NoError(t, err)
	assert.Empty(t, layer.GetCommands())
}
