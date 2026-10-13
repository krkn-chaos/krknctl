package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltSize        = 16
	nonceSize       = 12 // AES-GCM standard nonce
	keySize         = 32 // AES-256
	pbkdf2Iter      = 600_000
	encryptedSuffix = ".enc"
)

// magicHeader identifies a krkn encrypted archive. The trailing byte is a
// format version so future changes can be detected.
var magicHeader = []byte("KRKN-ENC\x01")

// IsEncryptedArchive reports whether the file at path begins with the krkn
// encrypted-archive magic header.
func IsEncryptedArchive(path string) bool {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return false
	}
	defer root.Close()

	file, err := root.Open(filepath.Base(path))
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, len(magicHeader))
	if _, err := io.ReadFull(file, header); err != nil {
		return false
	}
	for i, b := range magicHeader {
		if header[i] != b {
			return false
		}
	}
	return true
}

func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iter, keySize, sha256.New)
}

// EncryptFile encrypts srcPath with AES-256-GCM using a key derived from
// passphrase via PBKDF2 and writes the result to dstPath.
// Format: magic (9 B) || salt (16 B) || nonce (12 B) || ciphertext+tag.
func EncryptFile(srcPath, dstPath, passphrase string) error {
	plaintext, err := readFileRooted(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read archive for encryption: %w", err)
	}

	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, len(magicHeader)+saltSize+nonceSize+len(ciphertext))
	out = append(out, magicHeader...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	dstRoot, err := os.OpenRoot(filepath.Dir(dstPath))
	if err != nil {
		return fmt.Errorf("failed to open destination directory: %w", err)
	}
	defer dstRoot.Close()

	f, err := dstRoot.OpenFile(filepath.Base(dstPath), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("failed to create encrypted file: %w", err)
	}
	written, err := f.Write(out)
	if err != nil {
		_ = f.Close()
		_ = dstRoot.Remove(filepath.Base(dstPath))
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}
	if written != len(out) {
		_ = f.Close()
		_ = dstRoot.Remove(filepath.Base(dstPath))
		return fmt.Errorf("failed to write encrypted file: short write (%d of %d bytes)", written, len(out))
	}
	if err := f.Close(); err != nil {
		_ = dstRoot.Remove(filepath.Base(dstPath))
		return fmt.Errorf("failed to close encrypted file: %w", err)
	}
	return nil
}

// DecryptFile decrypts srcPath using AES-256-GCM with a key derived from
// passphrase via PBKDF2 and writes the plaintext to dstPath.
func DecryptFile(srcPath, dstPath, passphrase string) error {
	data, err := readFileRooted(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted archive: %w", err)
	}

	if len(data) < len(magicHeader) {
		return fmt.Errorf("file does not appear to be an encrypted archive (too small)")
	}
	for i, b := range magicHeader {
		if data[i] != b {
			return fmt.Errorf("file does not appear to be an encrypted archive; if this is a plain .tar.gz, remove the --passphrase flag")
		}
	}
	data = data[len(magicHeader):]

	minSize := saltSize + nonceSize + aes.BlockSize
	if len(data) < minSize {
		return fmt.Errorf("encrypted archive is truncated")
	}

	salt := data[:saltSize]
	nonce := data[saltSize : saltSize+nonceSize]
	ciphertext := data[saltSize+nonceSize:]

	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed (wrong passphrase or corrupted archive): %w", err)
	}

	dstRoot, err := os.OpenRoot(filepath.Dir(dstPath))
	if err != nil {
		return fmt.Errorf("failed to open destination directory: %w", err)
	}
	defer dstRoot.Close()

	f, err := dstRoot.OpenFile(filepath.Base(dstPath), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("failed to create decrypted file: %w", err)
	}
	written, err := f.Write(plaintext)
	if err != nil {
		_ = f.Close()
		_ = dstRoot.Remove(filepath.Base(dstPath))
		return fmt.Errorf("failed to write decrypted file: %w", err)
	}
	if written != len(plaintext) {
		_ = f.Close()
		_ = dstRoot.Remove(filepath.Base(dstPath))
		return fmt.Errorf("failed to write decrypted file: short write (%d of %d bytes)", written, len(plaintext))
	}
	if err := f.Close(); err != nil {
		_ = dstRoot.Remove(filepath.Base(dstPath))
		return fmt.Errorf("failed to close decrypted file: %w", err)
	}
	return nil
}

func readFileRooted(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(filepath.Base(path))
}
