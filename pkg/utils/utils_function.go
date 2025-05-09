package utils

import (
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
	if filepath.IsAbs(folder) == false {
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
