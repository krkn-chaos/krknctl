package provider

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type batchSignatureProvider struct {
	calls atomic.Int32
}

func (p *batchSignatureProvider) GetRegistryImages(*models.RegistryV2) (*[]models.ScenarioTag, error) {
	return nil, nil
}

func (p *batchSignatureProvider) GetGlobalEnvironment(*models.RegistryV2, string) (*models.ScenarioDetail, error) {
	return nil, nil
}

func (p *batchSignatureProvider) GetScenarioDetail(string, *models.RegistryV2) (*models.ScenarioDetail, error) {
	return nil, nil
}

func (p *batchSignatureProvider) GetImageSignatureStatus(_ context.Context, _ *models.RegistryV2, _ models.ScenarioTag) (verify.SignatureStatus, error) {
	p.calls.Add(1)
	return verify.SignatureSigned, nil
}

func (p *batchSignatureProvider) ScaffoldScenarios([]string, bool, *models.RegistryV2, bool, *ScaffoldSeed) (*string, error) {
	return nil, nil
}

func TestVerifyImageSignaturesDeduplicatesDigestTags(t *testing.T) {
	dataProvider := &batchSignatureProvider{}
	digest := "sha256:shared"
	tags := []models.ScenarioTag{
		{Name: "latest", Digest: &digest},
		{Name: "stable", Digest: &digest},
	}

	statuses, err := VerifyImageSignatures(context.Background(), dataProvider, nil, tags)
	require.NoError(t, err)
	assert.Equal(t, []verify.SignatureStatus{verify.SignatureSigned, verify.SignatureSigned}, statuses)
	assert.Equal(t, int32(1), dataProvider.calls.Load())
}

func TestVerifyImageSignaturesReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := VerifyImageSignatures(ctx, &batchSignatureProvider{}, nil, []models.ScenarioTag{{Name: "scenario"}})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVerifyImageSignaturesMapsProviderErrorsToUnknown(t *testing.T) {
	dataProvider := &errorSignatureProvider{}
	statuses, err := VerifyImageSignatures(context.Background(), dataProvider, nil, []models.ScenarioTag{{Name: "scenario"}})
	require.NoError(t, err)
	assert.Equal(t, []verify.SignatureStatus{verify.SignatureUnknown}, statuses)
}

type errorSignatureProvider struct{ batchSignatureProvider }

func (p *errorSignatureProvider) GetImageSignatureStatus(context.Context, *models.RegistryV2, models.ScenarioTag) (verify.SignatureStatus, error) {
	return verify.SignatureUnknown, errors.New("verification unavailable")
}
