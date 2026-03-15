package article

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ImageUploader uploads an image file and returns the hatena:syntax value.
type ImageUploader func(ctx context.Context, filePath string) (string, error)

// imageRegexp matches Markdown image syntax including an optional title attribute:
//
//	![alt](path)
//	![alt](path "title")
var imageRegexp = regexp.MustCompile(`!\[([^\]]*)\]\(([^\s)]+)(?:\s+"[^"]*")?\)`)

// HasLocalImages reports whether body contains any local (non-http/https) image references.
func HasLocalImages(body string) bool {
	for _, sub := range imageRegexp.FindAllStringSubmatch(body, -1) {
		if len(sub) < 3 {
			continue
		}
		path := sub[2]
		if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
			return true
		}
	}
	return false
}

// ReplaceLocalImages scans body for Markdown image references and replaces
// local file paths (those not starting with http:// or https://) with the
// hatena:syntax value returned by uploader. Remote URLs are left unchanged.
// Paths that resolve outside baseDir (e.g. absolute paths or "../" escapes) are rejected.
func ReplaceLocalImages(ctx context.Context, body, baseDir string, uploader ImageUploader) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve baseDir %s: %w", baseDir, err)
	}

	var rerr error
	result := imageRegexp.ReplaceAllStringFunc(body, func(match string) string {
		if rerr != nil {
			return match
		}
		sub := imageRegexp.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		path := sub[2]
		if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
			return match
		}
		// Reject absolute paths and paths that escape baseDir via "../".
		if filepath.IsAbs(path) {
			rerr = fmt.Errorf("image path %s must be relative", path)
			return match
		}
		absPath, aerr := filepath.Abs(filepath.Join(absBase, path))
		if aerr != nil {
			rerr = fmt.Errorf("resolve image path %s: %w", path, aerr)
			return match
		}
		if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
			rerr = fmt.Errorf("image path %s is outside article directory", path)
			return match
		}
		syntax, err := uploader(ctx, absPath)
		if err != nil {
			rerr = fmt.Errorf("upload image %s: %w", path, err)
			return match
		}
		return syntax
	})
	if rerr != nil {
		return "", rerr
	}
	return result, nil
}
