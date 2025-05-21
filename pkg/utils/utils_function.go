package utils

import (
	"crypto/rand"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

func ExpandFolder(folder string, basePath *string) (*string, error) {
	if strings.HasPrefix(folder, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		replacedHome := strings.Replace(folder, "~/", "", 1)
		expandedPath := filepath.Join(home, replacedHome)
		return &expandedPath, nil
	}
	if !filepath.IsAbs(folder) {
		if basePath != nil {
			path := filepath.Join(*basePath, folder)
			return &path, nil
		} else {
			path, err := filepath.Abs(folder)
			if err != nil {
				return nil, err
			}
			return &path, nil
		}

	}
	return &folder, nil
}

func RandomInt64(max *int64) int64 {
	maxRand := int64(math.MaxInt64)
	if max != nil {
		maxRand = *max
	}
	bigRand, err := rand.Int(rand.Reader, big.NewInt(int64(maxRand)))
	if err != nil {
		panic(err)
	}
	return bigRand.Int64()
}
