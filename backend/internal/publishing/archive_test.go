package publishing

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type zipEntry struct {
	name string
	data []byte
	mode os.FileMode
}

func testPNG(t *testing.T, value uint8) []byte {
	t.Helper()
	var out bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, color.RGBA{value, 0, 0, 255})
	if err := png.Encode(&out, im); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func testZIP(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func TestArchiveValidation(t *testing.T) {
	markdown := zipEntry{"notes/中文.md", []byte("# 中文\n![图片](../images/a.png)"), 0}
	image := zipEntry{"images/a.png", testPNG(t, 7), 0}
	pkg, err := readPackage(testZIP(t, markdown, image), "article.zip")
	if err != nil || pkg.Filename != markdown.name || len(pkg.Images) != 1 {
		t.Fatalf("valid ZIP: %+v %v", pkg, err)
	}
	for _, entries := range [][]zipEntry{
		{image}, {markdown, {"second.md", []byte("# Two"), 0}},
		{markdown, {"../escape.png", image.data, 0}}, {markdown, {"/absolute.png", image.data, 0}},
		{markdown, {"C:/windows.png", image.data, 0}}, {markdown, {"a\\b.png", image.data, 0}},
		{markdown, {"link.png", []byte("target"), os.ModeSymlink | 0777}},
		{markdown, {"bad.svg", []byte("<svg/>"), 0}}, {markdown, {"fake.png", []byte("<html>"), 0}},
		{markdown, {"wrong.jpg", image.data, 0}}, {markdown, image, {"images/A.png", image.data, 0}},
		{markdown, {"oversize.png", bytes.Repeat([]byte{0}, MaxImage+1), 0}},
		{markdown, {"bad/../image.png", image.data, 0}},
	} {
		if _, err := readPackage(testZIP(t, entries...), "bad.zip"); err == nil {
			t.Fatalf("unsafe ZIP accepted: %v", entries[0].name)
		}
	}
	entries := []zipEntry{markdown}
	for i := 0; i < MaxArchiveFiles; i++ {
		entries = append(entries, zipEntry{uuid.NewString() + ".png", image.data, 0})
	}
	if _, err := readPackage(testZIP(t, entries...), "many.zip"); err == nil {
		t.Fatal("too many entries accepted")
	}
	encrypted := testZIP(t, markdown, image)
	central := bytes.Index(encrypted, []byte{'P', 'K', 1, 2})
	binary.LittleEndian.PutUint16(encrypted[central+8:central+10], 1)
	if _, err := readPackage(encrypted, "encrypted.zip"); err == nil {
		t.Fatal("encrypted ZIP accepted")
	}
	corrupt := testZIP(t, markdown, image)
	central = bytes.Index(corrupt, []byte{'P', 'K', 1, 2})
	corrupt[central+16] ^= 255
	if _, err := readPackage(corrupt, "corrupt.zip"); err == nil {
		t.Fatal("bad CRC accepted")
	}
}

func TestImageDecodingAndPixelBounds(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var jpegBytes, gifBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, im, nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&gifBytes, im, nil); err != nil {
		t.Fatal(err)
	}
	webp, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"tiny.png": testPNG(t, 1), "tiny.jpg": jpegBytes.Bytes(), "tiny.gif": gifBytes.Bytes(), "tiny.webp": webp} {
		if media, err := validateImage(data, name); err != nil || media != imageType(name) {
			t.Fatalf("valid %s: %v", name, err)
		}
	}
	huge := testPNG(t, 1)
	binary.BigEndian.PutUint32(huge[16:20], 20000)
	binary.BigEndian.PutUint32(huge[20:24], 20000)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if _, err := validateImage(huge, "huge.png"); err == nil {
		t.Fatal("oversized decoded image accepted")
	}
	if _, err := validateImage(testPNG(t, 1)[:40], "truncated.png"); err == nil {
		t.Fatal("truncated image accepted")
	}
	entries := []zipEntry{{"post.md", []byte("# Post"), 0}}
	for i := 0; i < 11; i++ {
		entries = append(entries, zipEntry{uuid.NewString() + ".png", testPNG(t, 1), 0})
	}
	zipData := testZIP(t, entries...)
	offset := 0
	for i := 0; i < len(entries); i++ {
		central := bytes.Index(zipData[offset:], []byte{'P', 'K', 1, 2}) + offset
		if i > 0 {
			binary.LittleEndian.PutUint32(zipData[central+24:central+28], MaxImage)
		}
		offset = central + 46
	}
	if _, err := readPackage(zipData, "expanded.zip"); err == nil || err.Error() != "archive_expansion_limit" {
		t.Fatalf("expanded-size bound: %v", err)
	}
}

func TestImageReferencesStayWithinPackage(t *testing.T) {
	for _, value := range []string{"../../outside.png", "/etc/private.png", "file:///etc/private.png", "https://host/a.png", "//host/a.png", "..%2f..%2fsecret.png", "a.png?token=1", "a.png#id", "C:\\private.png"} {
		if _, ok := localImagePath("notes/post.md", value); ok {
			t.Fatalf("unsafe reference %q", value)
		}
	}
	for _, value := range []string{"../images/a.png", "../images/a%20b.png", "../images/中文.png"} {
		if _, ok := localImagePath("notes/post.md", value); !ok {
			t.Fatalf("safe reference %q", value)
		}
	}
}

func TestNoAssetRouteCanBeForged(t *testing.T) {
	id, assetID := uuid.NewString(), uuid.NewString()
	title, language, date, slug := "Title", "en", "2026-11-07", "title"
	revision := &Revision{ArticleID: id, Title: &title, Language: &language, WrittenAt: &date, Markdown: "![image](" + assetURL(id, assetID) + ")"}
	if !blocking(revisionIssues(revision, &slug)) {
		t.Fatal("route string authorized missing membership")
	}
	revision.Assets = []asset{{ID: assetID, ArticleID: uuid.NewString()}}
	if !blocking(revisionIssues(revision, &slug)) {
		t.Fatal("cross-article member accepted")
	}
	revision.Assets[0].ArticleID = id
	if blocking(revisionIssues(revision, &slug)) {
		t.Fatal("owned asset blocked")
	}
}

func TestAttachImagesRewritesOnlyParsedDestinations(t *testing.T) {
	// Rewriting itself is exercised with a real transaction by integration tests.
	// Without package images, original text and code examples remain unchanged.
	s, err := NewService(nil, t.TempDir()+"/uploads")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	original := "# Examples\n\n```md\n![sample](a.png)\n```\n\n![missing](a.png)"
	r := &Revision{ArticleID: uuid.NewString(), Markdown: original}
	if err := s.attachImages(context.Background(), nil, r, articlePackage{Filename: "post.md"}); err != nil || r.Markdown != original {
		t.Fatal("unresolved or code source changed")
	}
	if _, err := readPackage([]byte(strings.Repeat("a", MaxMarkdown+1)), "large.md"); err == nil {
		t.Fatal("large Markdown accepted")
	}
}
