package preview

import (
	"bytes"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type Kind string

const (
	InlineImage Kind = "inline-image"
	InlineAudio Kind = "inline-audio"
	InlineVideo Kind = "inline-video"
	InlinePDF   Kind = "inline-pdf"
	Text        Kind = "text"
	Download    Kind = "download"
)

type Result struct {
	Kind         Kind
	ContentType  string
	SafeInline   bool
	SniffedType  string
	ExpectedType string
}

func Classify(filename, clientType string, head []byte) Result {
	clientType = normalizeMIME(clientType)
	sniffed := http.DetectContentType(head)
	expected := normalizeMIME(mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))))
	ext := strings.ToLower(filepath.Ext(filename))

	if expected == "" {
		expected = clientType
	}
	if isExplicitlyActive(ext, expected, sniffed) {
		return download(clientType, sniffed, expected)
	}
	if isTextExtension(ext) && strings.HasPrefix(sniffed, "text/") {
		contentType := expected
		if contentType == "" {
			contentType = "text/plain; charset=utf-8"
		}
		return Result{Kind: Text, ContentType: contentType, SniffedType: sniffed, ExpectedType: expected}
	}
	if expected != "" && sniffed != "" && expected == sniffed {
		switch {
		case strings.HasPrefix(expected, "image/") && expected != "image/svg+xml":
			return Result{Kind: InlineImage, ContentType: expected, SafeInline: true, SniffedType: sniffed, ExpectedType: expected}
		case strings.HasPrefix(expected, "audio/"):
			return Result{Kind: InlineAudio, ContentType: expected, SafeInline: true, SniffedType: sniffed, ExpectedType: expected}
		case strings.HasPrefix(expected, "video/"):
			return Result{Kind: InlineVideo, ContentType: expected, SafeInline: true, SniffedType: sniffed, ExpectedType: expected}
		case expected == "application/pdf":
			return Result{Kind: InlinePDF, ContentType: expected, SafeInline: true, SniffedType: sniffed, ExpectedType: expected}
		}
	}
	contentType := expected
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "application/octet-stream"
	}
	return Result{Kind: Download, ContentType: contentType, SniffedType: sniffed, ExpectedType: expected}
}

func normalizeMIME(value string) string {
	value = strings.TrimSpace(strings.Split(value, ";")[0])
	if value == "" {
		return ""
	}
	normalized, _, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return normalized
}

func isTextExtension(ext string) bool {
	switch ext {
	case ".txt", ".md", ".markdown", ".log", ".sql", ".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".csv":
		return true
	default:
		return false
	}
}

func isExplicitlyActive(ext, expected, sniffed string) bool {
	values := []string{ext, expected, sniffed}
	for _, value := range values {
		value = strings.ToLower(value)
		if value == ".svg" || value == ".html" || value == ".htm" || value == ".xml" ||
			value == "image/svg+xml" || value == "text/html" || strings.HasPrefix(value, "application/xml") ||
			strings.HasPrefix(value, "text/xml") {
			return true
		}
	}
	return false
}

func download(clientType, sniffed, expected string) Result {
	contentType := "application/octet-stream"
	if clientType != "" && clientType != "application/octet-stream" {
		contentType = clientType
	}
	return Result{Kind: Download, ContentType: contentType, SniffedType: sniffed, ExpectedType: expected}
}

func IsPDF(head []byte) bool {
	return bytes.HasPrefix(head, []byte("%PDF-"))
}
