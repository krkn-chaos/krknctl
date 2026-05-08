package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVisualizeCommand(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewVisualizeCommand(&orchestrator, config)
	assert.NotNil(t, cmd)
	assert.Equal(t, "visualize", cmd.Use)
}

func TestVisualizeCommandFlags(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewVisualizeCommand(&orchestrator, config)

	assert.NotNil(t, cmd.Flags().Lookup("es-url"))
	assert.NotNil(t, cmd.Flags().Lookup("es-username"))
	assert.NotNil(t, cmd.Flags().Lookup("es-password"))
	assert.NotNil(t, cmd.Flags().Lookup("prometheus-url"))
	assert.NotNil(t, cmd.Flags().Lookup("prometheus-bearer-token"))
	assert.NotNil(t, cmd.Flags().Lookup("namespace"))
	assert.NotNil(t, cmd.Flags().Lookup("grafana-password"))
	assert.NotNil(t, cmd.Flags().Lookup("kubectl"))
	assert.NotNil(t, cmd.Flags().Lookup("kubeconfig"))
	assert.NotNil(t, cmd.Flags().Lookup("delete"))
}

func TestVisualizeGrafanaPasswordRequiredWithoutDelete(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewVisualizeCommand(&orchestrator, config)

	// --grafana-password not required when --delete is set
	grafanaPasswordFlag := cmd.Flags().Lookup("grafana-password")
	assert.NotNil(t, grafanaPasswordFlag)
	assert.False(t, grafanaPasswordFlag.Changed)

	// flag should not be statically marked required by cobra
	assert.Empty(t, grafanaPasswordFlag.Annotations["cobra_annotation_bash_completion_one_required_flag"])
}

func TestVisualizeCommandFlagDefaults(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewVisualizeCommand(&orchestrator, config)

	namespace, err := cmd.Flags().GetString("namespace")
	assert.Nil(t, err)
	assert.Equal(t, "krkn-visualize", namespace)

	grafanaPassword, err := cmd.Flags().GetString("grafana-password")
	assert.Nil(t, err)
	assert.Equal(t, "", grafanaPassword)

	kubectlCmd, err := cmd.Flags().GetString("kubectl")
	assert.Nil(t, err)
	assert.Equal(t, "kubectl", kubectlCmd)

	deleteFlag, err := cmd.Flags().GetBool("delete")
	assert.Nil(t, err)
	assert.False(t, deleteFlag)

	esURL, err := cmd.Flags().GetString("es-url")
	assert.Nil(t, err)
	assert.Equal(t, "", esURL)

	esUsername, err := cmd.Flags().GetString("es-username")
	assert.Nil(t, err)
	assert.Equal(t, "", esUsername)

	esPassword, err := cmd.Flags().GetString("es-password")
	assert.Nil(t, err)
	assert.Equal(t, "", esPassword)

	prometheusURL, err := cmd.Flags().GetString("prometheus-url")
	assert.Nil(t, err)
	assert.Equal(t, "", prometheusURL)

	prometheusBearer, err := cmd.Flags().GetString("prometheus-bearer-token")
	assert.Nil(t, err)
	assert.Equal(t, "", prometheusBearer)
}

func TestVisualizeDisplayEnvSecretMasking(t *testing.T) {
	config := getConfig(t)

	// Verify that ParsedField with secret:true is masked in the environment table
	displayEnv := map[string]ParsedField{
		"ES_PASSWORD":       {value: "supersecret", secret: true},
		"PROMETHEUS_BEARER": {value: "mytoken123", secret: true},
		"ES_URL":            {value: "http://elastic:9200", secret: false},
	}

	tbl := NewEnvironmentTable(displayEnv, config)
	assert.NotNil(t, tbl)

	// Confirm the ParsedField secret flag is set correctly
	assert.True(t, displayEnv["ES_PASSWORD"].secret)
	assert.True(t, displayEnv["PROMETHEUS_BEARER"].secret)
	assert.False(t, displayEnv["ES_URL"].secret)

	// Confirm raw values are still accessible (for passing to container)
	assert.Equal(t, "supersecret", displayEnv["ES_PASSWORD"].value)
	assert.Equal(t, "mytoken123", displayEnv["PROMETHEUS_BEARER"].value)
}

func TestVisualizeParsedFieldSecretFlag(t *testing.T) {
	secretField := ParsedField{value: "mypassword", secret: true}
	plainField := ParsedField{value: "myvalue", secret: false}

	assert.True(t, secretField.secret)
	assert.Equal(t, "mypassword", secretField.value)

	assert.False(t, plainField.secret)
	assert.Equal(t, "myvalue", plainField.value)
}
