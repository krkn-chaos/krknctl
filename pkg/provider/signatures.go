package provider

import (
	"context"
	"sync"

	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
)

const signatureWorkers = 8

// VerifyImageSignatures verifies a batch of scenario tags with bounded
// concurrency. Tags sharing a digest are verified once and receive the same
// result. Operational verification failures are represented as unknown.
func VerifyImageSignatures(ctx context.Context, dataProvider ScenarioDataProvider, registry *models.RegistryV2, tags []models.ScenarioTag) ([]verify.SignatureStatus, error) {
	statuses := make([]verify.SignatureStatus, len(tags))
	if len(tags) == 0 {
		return statuses, nil
	}

	unique := make(map[string]models.ScenarioTag)
	keys := make([]string, len(tags))
	for i, tag := range tags {
		key := tag.Name
		if tag.Digest != nil && *tag.Digest != "" {
			key = "digest:" + *tag.Digest
		}
		keys[i] = key
		unique[key] = tag
	}

	type result struct {
		key    string
		status verify.SignatureStatus
	}
	jobs := make(chan string)
	results := make(chan result, len(unique))
	workerCount := signatureWorkers
	if len(unique) < workerCount {
		workerCount = len(unique)
	}
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case key, ok := <-jobs:
					if !ok {
						return
					}
					status, err := dataProvider.GetImageSignatureStatus(ctx, registry, unique[key])
					if err != nil || status == "" {
						status = verify.SignatureUnknown
					}
					results <- result{key: key, status: status}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for key := range unique {
			select {
			case <-ctx.Done():
				return
			case jobs <- key:
			}
		}
	}()
	workers.Wait()
	close(results)
	resolved := make(map[string]verify.SignatureStatus, len(unique))
	for item := range results {
		resolved[item.key] = item.status
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i, key := range keys {
		statuses[i] = resolved[key]
	}
	return statuses, nil
}
