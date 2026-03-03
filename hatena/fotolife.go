package hatena

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"mime"
	"os"
	"path/filepath"
)

type fotolifeEntry struct {
	XMLName  xml.Name        `xml:"http://purl.org/atom/ns# entry"`
	Title    string          `xml:"title"`
	Content  fotolifeContent `xml:"content"`
}

type fotolifeContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type fotolifeResponse struct {
	XMLName xml.Name `xml:"http://purl.org/atom/ns# entry"`
	Syntax  string   `xml:"http://www.hatena.ne.jp/info/xmlns# syntax"`
}

// maxImageSize is the upload size limit enforced before reading the file into memory.
const maxImageSize = 10 * 1024 * 1024 // 10 MB

// UploadImage uploads the image at filePath to Hatena Fotolife and returns the hatena:syntax value.
func (c *Client) UploadImage(ctx context.Context, filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("stat image %s: %w", filePath, err)
	}
	if info.Size() > maxImageSize {
		return "", fmt.Errorf("image %s exceeds maximum size of %d bytes", filePath, maxImageSize)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read image %s: %w", filePath, err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	entry := fotolifeEntry{
		Title: filepath.Base(filePath),
		Content: fotolifeContent{
			Type:  mimeType,
			Value: base64.StdEncoding.EncodeToString(data),
		},
	}
	body, err := xml.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal fotolife entry: %w", err)
	}

	resp, err := c.do(ctx, "POST", c.fotolifeURL, body)
	if err != nil {
		return "", err
	}
	rawBody, err := readBody(resp)
	if err != nil {
		return "", err
	}
	if err := checkStatus(resp, rawBody); err != nil {
		return "", err
	}

	var result fotolifeResponse
	if err := xml.Unmarshal(rawBody, &result); err != nil {
		return "", fmt.Errorf("parse fotolife response: %w", err)
	}
	if result.Syntax == "" {
		return "", fmt.Errorf("hatena:syntax not found in fotolife response")
	}
	return result.Syntax, nil
}
