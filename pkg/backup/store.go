package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/krkn-chaos/krknctl/pkg/backupstore"
	"k8s.io/client-go/dynamic"
)

// BackupResult describes the outcome of a backup operation.
type BackupResult struct {
	ArchivePath string
	Location    *backupstore.Location
}

// CreateBackupAndStore creates a backup archive and optionally uploads it to
// the provided store. When store is nil the archive is only written locally.
func CreateBackupAndStore(ctx context.Context, dynClient dynamic.Interface, config BackupConfig, store backupstore.Store) (BackupResult, error) {
	archivePath, err := CreateBackup(ctx, dynClient, config)
	if err != nil {
		return BackupResult{}, err
	}

	if store == nil {
		return BackupResult{ArchivePath: archivePath}, nil
	}

	uploadFile, err := os.Open(archivePath) // #nosec G304 -- path from CreateBackup
	if err != nil {
		return BackupResult{ArchivePath: archivePath}, fmt.Errorf("failed to open archive for upload: %w", err)
	}
	defer uploadFile.Close()

	fileInfo, err := uploadFile.Stat()
	if err != nil {
		return BackupResult{ArchivePath: archivePath}, fmt.Errorf("failed to stat archive: %w", err)
	}

	objectKey := filepath.Base(archivePath)
	loc, err := store.Put(ctx, objectKey, uploadFile, fileInfo.Size())
	if err != nil {
		return BackupResult{ArchivePath: archivePath}, fmt.Errorf("failed to upload archive: %w", err)
	}

	return BackupResult{
		ArchivePath: archivePath,
		Location:    &loc,
	}, nil
}
