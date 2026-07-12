package main

import (
	"path/filepath"
	"strings"
	"unicode"
)

func cleanDisplayName(name string) string {
	name = stripWeirdRunes(name)
	return strings.TrimSpace(name)
}

func stripWeirdRunes(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if r == '\uFFFD' {
			continue
		}
		if r < 32 || r == 127 {
			continue
		}
		if r >= 0x80 && r <= 0x9F {
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func sanitizePathName(name string) string {
	name = sanitizeNamePart(name)
	if name == "" {
		return "album"
	}
	return name
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." {
		return "download.bin"
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	ext = strings.TrimPrefix(ext, ".")

	base = sanitizeNamePart(base)
	if base == "" {
		base = "download"
	}
	if ext != "" {
		ext = sanitizeNamePart(ext)
		if ext != "" {
			return base + "." + ext
		}
	}
	return base
}

func sanitizeNamePart(value string) string {
	value = stripWeirdRunes(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_', r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteByte('_')
		default:
			continue
		}
	}

	out := collapseUnderscores(strings.Trim(b.String(), "._-"))
	return out
}

func collapseUnderscores(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	prevUnderscore := false
	for _, r := range value {
		if r == '_' {
			if prevUnderscore {
				continue
			}
			prevUnderscore = true
			b.WriteRune(r)
			continue
		}
		prevUnderscore = false
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), "_")
}

func downloadDestPath(outputFolder, albumTitle string, file AlbumFile) string {
	return filepath.Join(outputFolder, sanitizePathName(albumTitle), sanitizeFileName(file.Name))
}
