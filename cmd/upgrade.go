package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/spf13/cobra"
)

// GitHubReleaseAsset represents a single asset in a GitHub release
type GitHubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// GitHubReleaseInfo represents the GitHub API response for a release
type GitHubReleaseInfo struct {
	TagName string               `json:"tag_name"`
	Assets  []GitHubReleaseAsset `json:"assets"`
}

// NewUpgradeCommand creates the upgrade command
func NewUpgradeCommand(krknctlConfig config.Config) *cobra.Command {
	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade krknctl to the latest version",
		Long:  `Automatically downloads and installs the latest version of krknctl from GitHub releases`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(krknctlConfig)
		},
	}
	return upgradeCmd
}

// runUpgrade executes the upgrade process
func runUpgrade(cfg config.Config) error {
	fmt.Println(color.CyanString("🔍 Checking for latest version..."))

	// Fetch latest release information from GitHub API
	latestVersion, assets, err := fetchLatestRelease(cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	// Compare versions
	currentVersion := cfg.Version
	if currentVersion == latestVersion {
		fmt.Println(color.GreenString("✅ You are already using the latest version: %s", currentVersion))
		return nil
	}

	fmt.Println(color.YellowString("📦 New version available: %s (current: %s)", latestVersion, currentVersion))

	// Detect OS and architecture
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Find the appropriate binary asset
	assetName, downloadURL, err := findBinaryAsset(assets, osName, arch)
	if err != nil {
		return fmt.Errorf("failed to find binary for %s/%s: %w", osName, arch, err)
	}

	fmt.Println(color.CyanString("⬇️  Downloading %s...", assetName))

	// Download the binary
	binaryData, err := downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Find and verify checksum
	checksumAssetName := assetName + ".sha256"
	checksumURL, err := findChecksumAsset(assets, checksumAssetName)
	if err == nil {
		fmt.Println(color.CyanString("🔐 Verifying checksum..."))
		if err := verifyChecksum(binaryData, checksumURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		fmt.Println(color.GreenString("✅ Checksum verified"))
	} else {
		fmt.Println(color.YellowString("⚠️  No checksum found, skipping verification"))
	}

	fmt.Println(color.CyanString("🔄 Installing update..."))

	// Replace the current executable
	if err := replaceExecutable(binaryData); err != nil {
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	fmt.Println(color.GreenString("✅ krknctl successfully upgraded to %s", latestVersion))
	return nil
}

// fetchLatestRelease queries the GitHub API for the latest release
func fetchLatestRelease(cfg config.Config) (string, []GitHubReleaseAsset, error) {
	// Use the GitHub API to get latest release details
	apiURL := "https://api.github.com/repos/krkn-chaos/krknctl/releases/latest"

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var release GitHubReleaseInfo
	if err := json.Unmarshal(body, &release); err != nil {
		return "", nil, fmt.Errorf("failed to parse release JSON: %w", err)
	}

	if release.TagName == "" {
		return "", nil, fmt.Errorf("no tag_name found in release")
	}

	return release.TagName, release.Assets, nil
}

// findBinaryAsset locates the correct binary asset for the current OS and architecture
func findBinaryAsset(assets []GitHubReleaseAsset, osName, arch string) (string, string, error) {
	// Normalize OS name for matching
	var osPattern string
	switch osName {
	case "darwin":
		osPattern = "darwin"
	case "linux":
		osPattern = "linux"
	case "windows":
		osPattern = "windows"
	default:
		return "", "", fmt.Errorf("unsupported operating system: %s", osName)
	}

	// Normalize architecture for matching
	var archPattern string
	switch arch {
	case "amd64":
		archPattern = "amd64"
	case "arm64":
		archPattern = "arm64"
	case "386":
		archPattern = "386"
	default:
		return "", "", fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Search for matching asset
	// Expected naming pattern: krknctl-<os>-<arch> or krknctl-<os>-<arch>.exe
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)

		// Check if asset matches OS and architecture
		if strings.Contains(name, osPattern) && strings.Contains(name, archPattern) {
			// Verify it's a binary (not a checksum or other file)
			if !strings.HasSuffix(name, ".sha256") &&
				!strings.HasSuffix(name, ".txt") &&
				!strings.HasSuffix(name, ".md") {
				return asset.Name, asset.BrowserDownloadURL, nil
			}
		}
	}

	return "", "", fmt.Errorf("no matching binary found for %s/%s", osName, arch)
}

// findChecksumAsset locates the checksum file for the binary
func findChecksumAsset(assets []GitHubReleaseAsset, checksumName string) (string, error) {
	for _, asset := range assets {
		if strings.EqualFold(asset.Name, checksumName) {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("checksum file not found: %s", checksumName)
}

// verifyChecksum downloads and verifies the SHA256 checksum of the binary
func verifyChecksum(binaryData []byte, checksumURL string) error {
	// Download checksum file
	checksumData, err := downloadBinary(checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksum: %w", err)
	}

	// Parse checksum (format: "hash  filename" or just "hash")
	checksumStr := strings.TrimSpace(string(checksumData))
	expectedHash := strings.Fields(checksumStr)[0]

	// Compute actual hash
	hash := sha256.Sum256(binaryData)
	actualHash := hex.EncodeToString(hash[:])

	// Compare
	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// downloadBinary downloads the binary from the given URL
func downloadBinary(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 5 * time.Minute, // Allow time for larger downloads
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read download data: %w", err)
	}

	return data, nil
}

// replaceExecutable safely replaces the current executable with the new binary
func replaceExecutable(newBinary []byte) error {
	// Get the path of the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Create a temporary file in the same directory as the executable
	// This ensures we're on the same filesystem for atomic rename
	execDir := filepath.Dir(execPath)
	tempFile, err := os.CreateTemp(execDir, "krknctl-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup on error
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			os.Remove(tempPath)
		}
	}()

	// Write the new binary to the temporary file
	if _, err := tempFile.Write(newBinary); err != nil {
		return fmt.Errorf("failed to write new binary: %w", err)
	}

	// Close the temp file before changing permissions
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Set executable permissions (0755 = rwxr-xr-x)
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// On Windows, we can't replace a running executable directly
	// Write the new binary to a .new file and instruct the user
	if runtime.GOOS == "windows" {
		newPath := execPath + ".new"
		
		// Move new binary to .new path
		if err := os.Rename(tempPath, newPath); err != nil {
			return fmt.Errorf("failed to save new binary: %w", err)
		}

		// Mark tempFile as nil so defer doesn't try to clean it up
		tempFile = nil

		fmt.Println(color.YellowString("\n⚠️  Windows requires manual completion:"))
		fmt.Println(color.YellowString("   1. Close this terminal"))
		fmt.Println(color.YellowString("   2. Rename or delete: %s", execPath))
		fmt.Println(color.YellowString("   3. Rename %s to %s", newPath, execPath))
		fmt.Println(color.YellowString("   4. Restart krknctl\n"))
		
		return nil
	}

	// On Unix-like systems, we can atomically replace the file
	if err := os.Rename(tempPath, execPath); err != nil {
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	// Mark tempFile as nil so defer doesn't try to clean it up
	tempFile = nil

	return nil
}
