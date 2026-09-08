package cmd

import (
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/stretchr/testify/require"
)

func TestNewScenarioTable_AllowsMissingMetadata(t *testing.T) {
	scenarios := []models.ScenarioTag{{Name: "scenario-without-metadata"}}

	table := NewScenarioTable(&scenarios, false)
	require.NotNil(t, table)
}
