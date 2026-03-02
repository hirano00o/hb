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

var imageRegexp = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

// ReplaceLocalImages scans body for Markdown image references and replaces
// local file paths (those not starting with http:// or https://) with the
// hatena:syntax value returned by uploader. Remote URLs are left unchanged.
func ReplaceLocalImages(ctx context.Context, body, baseDir string, uploader ImageUploader) (string, error) {
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
		absPath := filepath.Join(baseDir, path)
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
