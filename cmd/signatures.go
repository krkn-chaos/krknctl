package cmd

import (
	"context"

	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)

// PopulateScenarioSignatureStatuses verifies the signature status of every
// listed scenario. Verification failures are represented as unknown so one
// unavailable registry entry does not hide the rest of the listing.
func PopulateScenarioSignatureStatuses(ctx context.Context, dataProvider provider.ScenarioDataProvider, registry *models.RegistryV2, scenarios *[]models.ScenarioTag) error {
	statuses, err := provider.VerifyImageSignatures(ctx, dataProvider, registry, *scenarios)
	if err != nil {
		return err
	}
	for i, status := range statuses {
		(*scenarios)[i].SignatureStatus = string(status)
	}
	return nil
}
