package tools

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	_ "image/gif"
	"os"
	"path/filepath"
	"strings"
)

// ReadImage reads an image file and returns metadata + optional base64 data
func ReadImage(path string) (string, map[string]interface{}, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("image file not found: %s", err)
	}

	// Get image dimensions
	width, height, format := 0, 0, ""
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		conf, fmtStr, err := image.DecodeConfig(f)
		if err == nil {
			width = conf.Width
			height = conf.Height
			format = fmtStr
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	if format == "" {
		switch ext {
		case ".png":
			format = "png"
		case ".jpg", ".jpeg":
			format = "jpeg"
		case ".gif":
			format = "gif"
		case ".webp":
			format = "webp"
		default:
			format = strings.TrimPrefix(ext, ".")
		}
	}

	// Build metadata
	metadata := map[string]interface{}{
		"path":       path,
		"size":       info.Size(),
		"width":      width,
		"height":     height,
		"format":     format,
		"image_path": path,
	}

	// Read and encode as base64 for vision model support
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("[Image: %s, %dx%d, %s, %d bytes]",
			filepath.Base(path), width, height, format, info.Size()), metadata, nil
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	metadata["base64"] = b64
	metadata["base64_length"] = len(b64)

	// Return text description and data
	desc := fmt.Sprintf("[Image: %s, %dx%d, %s, %d bytes]",
		filepath.Base(path), width, height, format, info.Size())
	if width > 0 && height > 0 {
		desc += fmt.Sprintf("\nDimensions: %dx%d", width, height)
	}
	desc += fmt.Sprintf("\nFormat: %s\nSize: %d bytes", format, info.Size())

	return desc, metadata, nil
}
