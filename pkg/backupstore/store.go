/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package backupstore provides optional persistence for operator backup archives.
// Archive creation and validation remain in krknctl/pkg/backup; this package only
// handles storing an already-created archive.
package backupstore

import (
	"context"
	"io"
)

// Store persists a backup archive and may return a temporary download URL.
type Store interface {
	Put(ctx context.Context, key string, source io.Reader, size int64) (Location, error)
}

// Reader is an optional capability for stores that can read an archive by
// object key. It is separate from Store so existing upload-only consumers do
// not need to implement restore support.
type Reader interface {
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
}

// Location identifies the persisted archive.
type Location struct {
	Key         string
	DownloadURL string
}
