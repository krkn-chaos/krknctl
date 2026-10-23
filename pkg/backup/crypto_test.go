package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := []byte("krkn-backup archive content for round-trip test")
	srcPath := filepath.Join(dir, "backup.tar.gz")
	if !assert.NoError(t, os.WriteFile(srcPath, original, 0600)) {
		return
	}

	encPath := filepath.Join(dir, "backup.tar.gz.enc")
	if !assert.NoError(t, EncryptFile(srcPath, encPath, "test-passphrase")) {
		return
	}

	encData, err := os.ReadFile(encPath)
	if !assert.NoError(t, err) {
		return
	}
	assert.NotEqual(t, original, encData, "encrypted data must differ from plaintext")
	assert.Greater(t, len(encData), len(magicHeader)+saltSize+nonceSize, "encrypted file must contain magic+salt+nonce+ciphertext")

	decPath := filepath.Join(dir, "decrypted.tar.gz")
	if !assert.NoError(t, DecryptFile(encPath, decPath, "test-passphrase")) {
		return
	}

	decData, err := os.ReadFile(decPath)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, original, decData)
}

func TestEncryptedFileHasMagicHeader(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "backup.tar.gz")
	if !assert.NoError(t, os.WriteFile(srcPath, []byte("data"), 0600)) {
		return
	}
	encPath := filepath.Join(dir, "backup.tar.gz.enc")
	if !assert.NoError(t, EncryptFile(srcPath, encPath, "pass")) {
		return
	}
	assert.True(t, IsEncryptedArchive(encPath))
	assert.False(t, IsEncryptedArchive(srcPath))
}

func TestDecryptWrongPassphrase(t *testing.T) {
	dir := t.TempDir()

	srcPath := filepath.Join(dir, "backup.tar.gz")
	if !assert.NoError(t, os.WriteFile(srcPath, []byte("secret data"), 0600)) {
		return
	}

	encPath := filepath.Join(dir, "backup.tar.gz.enc")
	if !assert.NoError(t, EncryptFile(srcPath, encPath, "correct-passphrase")) {
		return
	}

	decPath := filepath.Join(dir, "decrypted.tar.gz")
	err := DecryptFile(encPath, decPath, "wrong-passphrase")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wrong passphrase or corrupted")
}

func TestDecryptPlainFileGivesHelpfulError(t *testing.T) {
	dir := t.TempDir()

	plainPath := filepath.Join(dir, "backup.tar.gz")
	if !assert.NoError(t, os.WriteFile(plainPath, []byte("this is a plain gzip file with enough bytes to pass size check"), 0600)) {
		return
	}

	decPath := filepath.Join(dir, "decrypted.tar.gz")
	err := DecryptFile(plainPath, decPath, "any-passphrase")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not appear to be an encrypted archive")
	assert.Contains(t, err.Error(), "remove the --passphrase flag")
}

func TestDecryptTruncatedFile(t *testing.T) {
	dir := t.TempDir()

	encPath := filepath.Join(dir, "truncated.enc")
	if !assert.NoError(t, os.WriteFile(encPath, []byte("short"), 0600)) {
		return
	}

	decPath := filepath.Join(dir, "decrypted.tar.gz")
	err := DecryptFile(encPath, decPath, "any")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not appear to be an encrypted archive")
}

func TestDecryptTruncatedAfterMagic(t *testing.T) {
	dir := t.TempDir()

	encPath := filepath.Join(dir, "truncated.enc")
	if !assert.NoError(t, os.WriteFile(encPath, magicHeader, 0600)) {
		return
	}

	decPath := filepath.Join(dir, "decrypted.tar.gz")
	err := DecryptFile(encPath, decPath, "any")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "truncated")
}

func TestEncryptProducesDifferentOutputEachTime(t *testing.T) {
	dir := t.TempDir()

	srcPath := filepath.Join(dir, "backup.tar.gz")
	if !assert.NoError(t, os.WriteFile(srcPath, []byte("same content"), 0600)) {
		return
	}

	enc1 := filepath.Join(dir, "enc1")
	enc2 := filepath.Join(dir, "enc2")
	if !assert.NoError(t, EncryptFile(srcPath, enc1, "passphrase")) {
		return
	}
	if !assert.NoError(t, EncryptFile(srcPath, enc2, "passphrase")) {
		return
	}

	data1, err := os.ReadFile(enc1)
	if !assert.NoError(t, err) {
		return
	}
	data2, err := os.ReadFile(enc2)
	if !assert.NoError(t, err) {
		return
	}
	assert.NotEqual(t, data1, data2, "each encryption must use a fresh salt and nonce")
}

func TestEncryptedBackupIntegration(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "source", backupDirName)
	if !assert.NoError(t, os.MkdirAll(backupDir, 0700)) {
		return
	}
	if !assert.NoError(t, os.WriteFile(filepath.Join(backupDir, "test.json"), []byte(`{"apiVersion":"v1","kind":"List","items":[]}`), 0600)) {
		return
	}

	archivePath := filepath.Join(dir, "backup.tar.gz")
	f, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY, 0600)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NoError(t, createTarGzFile(f, filepath.Join(dir, "source"), backupDirName)) {
		_ = f.Close()
		return
	}
	if !assert.NoError(t, f.Close()) {
		return
	}

	encPath := filepath.Join(dir, "backup.tar.gz.enc")
	if !assert.NoError(t, EncryptFile(archivePath, encPath, "integration-pass")) {
		return
	}

	decPath := filepath.Join(dir, "decrypted.tar.gz")
	if !assert.NoError(t, DecryptFile(encPath, decPath, "integration-pass")) {
		return
	}

	extractDir := filepath.Join(dir, "extracted")
	if !assert.NoError(t, os.MkdirAll(extractDir, 0700)) {
		return
	}
	if !assert.NoError(t, extractTarGz(decPath, extractDir)) {
		return
	}

	foundDir, err := findBackupDir(extractDir)
	if !assert.NoError(t, err) {
		return
	}
	entries, err := os.ReadDir(foundDir)
	if !assert.NoError(t, err) {
		return
	}
	assert.Len(t, entries, 1)
	assert.Equal(t, "test.json", entries[0].Name())
}
