package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func createAssetPath(videoID uuid.UUID, mediaType string, assetsRoot string) string {
	var fileExtension string
	parts := strings.Split(mediaType, "/")
	if len(parts) == 2 {
		fileExtension = parts[1]
	} else {
		fileExtension = "png" // Default extension if parsing fails
	}

	return fmt.Sprintf("%s.%s", videoID, fileExtension)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}