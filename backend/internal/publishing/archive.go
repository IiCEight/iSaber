package publishing

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
)

const (
	MaxImage          = 10 * 1024 * 1024
	MaxArchive        = 50 * 1024 * 1024
	MaxExpanded       = 100 * 1024 * 1024
	MaxArchiveFiles   = 200
	MaxArchiveEntries = 400 // Allow directory entries in addition to the file budget.
	MaxImagePixels    = 16_000_000
)

type imageFile struct {
	Data      []byte
	MediaType string
}
type articlePackage struct {
	Markdown []byte
	Filename string
	Images   map[string]imageFile
}

func imageType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return ""
}

// Archive names are never passed to the filesystem. Reject ambiguity as well as
// traversal: Markdown references must identify one exact, portable ZIP entry.
func safeArchivePath(name string) bool {
	if name == "" || len(name) > 1024 || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func readPackage(data []byte, filename string) (articlePackage, error) {
	result := articlePackage{Filename: filename, Images: map[string]imageFile{}}
	if strings.EqualFold(path.Ext(filename), ".md") {
		if len(data) > MaxMarkdown {
			return result, errors.New("markdown_too_large")
		}
		result.Markdown = data
		return result, nil
	}
	if !strings.EqualFold(path.Ext(filename), ".zip") || len(data) > MaxArchive {
		return result, errors.New("invalid_archive")
	}
	if !boundedZIPDirectory(data) {
		return result, errors.New("invalid_archive")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(reader.File) > MaxArchiveEntries {
		return result, errors.New("invalid_archive")
	}
	names := map[string]bool{}
	var expanded uint64
	markdownCount := 0
	fileCount := 0
	// Validate declared sizes and names before reading any expanded bytes.
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		if !safeArchivePath(name) || names[strings.ToLower(name)] || file.Flags&65 != 0 || (!file.Mode().IsRegular() && file.Mode()&os.ModeType != os.ModeDir) {
			return result, errors.New("unsafe_archive_entry")
		}
		names[strings.ToLower(name)] = true
		if file.FileInfo().IsDir() {
			if file.UncompressedSize64 != 0 {
				return result, errors.New("invalid_archive")
			}
			continue
		}
		fileCount++
		if fileCount > MaxArchiveFiles {
			return result, errors.New("archive_file_limit")
		}
		maximum := uint64(MaxImage)
		if strings.EqualFold(path.Ext(name), ".md") {
			maximum = MaxMarkdown
			markdownCount++
		} else if imageType(name) == "" {
			return result, errors.New("unsupported_archive_file")
		}
		if file.UncompressedSize64 > maximum || file.UncompressedSize64 > MaxExpanded-expanded {
			return result, errors.New("archive_expansion_limit")
		}
		expanded += file.UncompressedSize64
	}
	if markdownCount != 1 {
		return result, errors.New("one_markdown_file_required")
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		opened, err := file.Open()
		if err != nil {
			return result, errors.New("invalid_archive")
		}
		maximum := int64(MaxImage)
		if strings.EqualFold(path.Ext(file.Name), ".md") {
			maximum = MaxMarkdown
		}
		content, readErr := io.ReadAll(io.LimitReader(opened, maximum+1))
		closeErr := opened.Close()
		if readErr != nil || closeErr != nil || int64(len(content)) > maximum || uint64(len(content)) != file.UncompressedSize64 {
			return result, errors.New("invalid_archive")
		}
		if strings.EqualFold(path.Ext(file.Name), ".md") {
			result.Markdown = content
			result.Filename = file.Name
			continue
		}
		media, err := validateImage(content, file.Name)
		if err != nil {
			return result, err
		}
		result.Images[file.Name] = imageFile{content, media}
	}
	return result, nil
}

// Bound directory parsing before archive/zip allocates one object per entry.
// ZIP64 and split archives are unnecessary for this deliberately small format.
func boundedZIPDirectory(data []byte) bool {
	for offset := len(data) - 22; offset >= 0 && offset >= len(data)-22-65535; offset-- {
		if !bytes.Equal(data[offset:offset+4], []byte{'P', 'K', 5, 6}) {
			continue
		}
		record := data[offset:]
		if len(record) != 22+int(binary.LittleEndian.Uint16(record[20:22])) {
			continue
		}
		count := binary.LittleEndian.Uint16(record[10:12])
		return binary.LittleEndian.Uint16(record[4:6]) == 0 && binary.LittleEndian.Uint16(record[6:8]) == 0 &&
			count > 0 && count <= MaxArchiveEntries && binary.LittleEndian.Uint16(record[8:10]) == count &&
			binary.LittleEndian.Uint32(record[12:16]) <= 1024*1024 && binary.LittleEndian.Uint32(record[16:20]) != 0xffffffff
	}
	return false
}

func validateImage(data []byte, filename string) (string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	media := "image/" + format
	if err != nil || media != imageType(filename) || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > MaxImagePixels {
		return "", errors.New("invalid_image")
	}
	if _, decoded, err := image.Decode(bytes.NewReader(data)); err != nil || decoded != format {
		return "", errors.New("invalid_image")
	}
	return media, nil
}
