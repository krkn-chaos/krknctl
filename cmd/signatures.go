package cmd

import (
	"context"
	"sync"

	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
)

const signatureWorkers = 8

// PopulateScenarioSignatureStatuses verifies the signature status of every
// listed scenario. Verification failures are represented as unknown so one
// unavailable registry entry does not hide the rest of the listing.
func PopulateScenarioSignatureStatuses(ctx context.Context, dataProvider provider.ScenarioDataProvider, registry *models.RegistryV2, scenarios *[]models.ScenarioTag) error {
	if len(*scenarios) == 0 {
		return nil
	}

	workerCount := signatureWorkers
	if len(*scenarios) < workerCount {
		workerCount = len(*scenarios)
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case index, ok := <-jobs:
					if !ok {
						return
					}
					status, err := dataProvider.GetImageSignatureStatus(ctx, registry, (*scenarios)[index])
					if err != nil || status == "" {
						status = verify.SignatureUnknown
					}
					(*scenarios)[index].SignatureStatus = string(status)
				}
			}
		}()
	}

	for index := range *scenarios {
		select {
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return ctx.Err()
		case jobs <- index:
		}
	}
	close(jobs)
	workers.Wait()
	return ctx.Err()
}
