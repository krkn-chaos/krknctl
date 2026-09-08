package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
	"github.com/stretchr/testify/require"
)

type signatureStatusProvider struct {
	statuses map[string]verify.SignatureStatus
}

func (p signatureStatusProvider) GetRegistryImages(*models.RegistryV2) (*[]models.ScenarioTag, error) {
	return nil, nil
}

func (p signatureStatusProvider) GetGlobalEnvironment(*models.RegistryV2, string) (*models.ScenarioDetail, error) {
	return nil, nil
}

func (p signatureStatusProvider) GetScenarioDetail(string, *models.RegistryV2) (*models.ScenarioDetail, error) {
	return nil, nil
}

func (p signatureStatusProvider) GetImageSignatureStatus(_ context.Context, _ *models.RegistryV2, tag models.ScenarioTag) (verify.SignatureStatus, error) {
	status, ok := p.statuses[tag.Name]
	if !ok {
		return verify.SignatureUnknown, errors.New("verification failed")
	}
	return status, nil
}

func (p signatureStatusProvider) ScaffoldScenarios([]string, bool, *models.RegistryV2, bool, *provider.ScaffoldSeed) (*string, error) {
	return nil, nil
}

func TestPopulateScenarioSignatureStatuses(t *testing.T) {
	scenarios := []models.ScenarioTag{{Name: "signed"}, {Name: "unsigned"}, {Name: "unknown"}}
	dataProvider := signatureStatusProvider{statuses: map[string]verify.SignatureStatus{
		"signed":   verify.SignatureSigned,
		"unsigned": verify.SignatureUnsigned,
	}}

	require.NoError(t, PopulateScenarioSignatureStatuses(context.Background(), dataProvider, nil, &scenarios))
	require.Equal(t, "signed", scenarios[0].SignatureStatus)
	require.Equal(t, "unsigned", scenarios[1].SignatureStatus)
	require.Equal(t, "unknown", scenarios[2].SignatureStatus)
}
