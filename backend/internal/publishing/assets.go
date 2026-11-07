package publishing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/IiCEight/iSaber/backend/internal/auth"
	"github.com/google/uuid"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
)

type asset struct {
	ID, ArticleID, StorageKey, MediaType, OriginalPath string
	Size                                               int64
	Checksum                                           []byte
}

func assetURL(article, id string) string { return "/api/v1/articles/" + article + "/assets/" + id }

// A route-shaped string alone never authorizes an asset. Revision membership is
// loaded separately from PostgreSQL and must match the owning article exactly.
func hasManagedImage(revision *Revision, destination string) bool {
	for _, item := range revision.Assets {
		if item.ArticleID == revision.ArticleID && destination == assetURL(item.ArticleID, item.ID) {
			return true
		}
	}
	return false
}

func (s *Service) writePrivate(key string, data []byte) error {
	file, err := s.root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr = directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func localImagePath(article, destination string) (string, bool) {
	parsed, err := url.Parse(destination)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.HasPrefix(parsed.Path, "/") || strings.ContainsAny(parsed.Path, "\\\x00:") {
		return "", false
	}
	// Parent segments can stay within a ZIP root (notes/post.md -> ../images/a.png).
	resolved := path.Clean(path.Join(path.Dir(article), parsed.Path))
	return resolved, safeArchivePath(resolved)
}

type replacement struct {
	start, stop int
	value       string
}

func (s *Service) attachImages(ctx context.Context, tx *sql.Tx, r *Revision, pkg articlePackage) error {
	source := []byte(r.Markdown)
	document := parser.New(parser.WithExtensions(extension.NewGFMParser())).Parse(source)
	edits := map[int]replacement{}
	attached := map[string]asset{}
	err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		img, ok := node.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		destination := img.Destination.Value(source)
		filename, ok := localImagePath(pkg.Filename, destination)
		if !ok {
			return ast.WalkContinue, nil
		}
		file, ok := pkg.Images[filename]
		if !ok {
			return ast.WalkContinue, nil
		}
		if img.Destination.IsOwned() {
			return ast.WalkStop, errors.New("image_source_unavailable")
		}
		position := img.Destination.Index()
		if position.Start < 0 || position.Stop > len(source) || position.Start >= position.Stop {
			return ast.WalkStop, errors.New("image_source_unavailable")
		}
		item, exists := attached[filename]
		if !exists {
			checksum := sha256.Sum256(file.Data)
			item = asset{ID: uuid.NewString(), ArticleID: r.ArticleID, MediaType: file.MediaType, Size: int64(len(file.Data)), Checksum: checksum[:], OriginalPath: filename}
			item.StorageKey = item.ID + ".asset"
			// Bytes must exist before any SQL pointer can make them visible. Retain an
			// orphan on uncertain rollback/commit; automated retention is separate work.
			if err := s.writePrivate(item.StorageKey, file.Data); err != nil {
				return ast.WalkStop, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO assets(id,article_id,storage_key,media_type,byte_size,checksum) VALUES($1,$2,$3,$4,$5,$6)`, item.ID, item.ArticleID, item.StorageKey, item.MediaType, item.Size, item.Checksum); err != nil {
				return ast.WalkStop, err
			}
			attached[filename] = item
			r.Assets = append(r.Assets, item)
		}
		edits[position.Start] = replacement{position.Start, position.Stop, assetURL(r.ArticleID, item.ID)}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return err
	}
	ordered := make([]replacement, 0, len(edits))
	for _, edit := range edits {
		ordered = append(ordered, edit)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].start > ordered[j].start })
	for _, edit := range ordered {
		r.Markdown = r.Markdown[:edit.start] + edit.value + r.Markdown[edit.stop:]
	}
	if len(r.Markdown) > MaxMarkdown {
		return errors.New("normalized_markdown_too_large")
	}
	return nil
}

func (s *Service) readAsset(item asset) ([]byte, error) {
	bytes, err := s.readStage(item.StorageKey, MaxImage+1)
	if err != nil || int64(len(bytes)) != item.Size {
		return nil, errors.New("asset_unavailable")
	}
	checksum := sha256.Sum256(bytes)
	if string(checksum[:]) != string(item.Checksum) {
		return nil, errors.New("asset_unavailable")
	}
	return bytes, nil
}

func (s *Service) checkAssets(r *Revision) error {
	for _, item := range r.Assets {
		if _, err := s.readAsset(item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) AssetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		article, err := canonicalID(r.PathValue("article_id"))
		if err != nil {
			reply(w, nil, fault(404, "not_found"))
			return
		}
		id, err := canonicalID(r.PathValue("asset_id"))
		if err != nil {
			reply(w, nil, fault(404, "not_found"))
			return
		}
		// Optional authentication supplies owner context only after session validation.
		_, owner := auth.OwnerFromContext(r.Context())
		var item asset
		err = s.db.QueryRowContext(r.Context(), `SELECT x.id,x.article_id,x.storage_key,x.media_type,x.byte_size,x.checksum
   FROM assets x JOIN articles a ON a.id=x.article_id WHERE x.id=$1 AND a.id=$2 AND
   EXISTS(SELECT 1 FROM revision_assets ra WHERE ra.asset_id=x.id AND ra.article_id=a.id
     AND ($3 OR (a.visibility='public' AND ra.revision_id=a.published_revision_id)))`, id, article, owner).
			Scan(&item.ID, &item.ArticleID, &item.StorageKey, &item.MediaType, &item.Size, &item.Checksum)
		if errors.Is(err, sql.ErrNoRows) {
			reply(w, nil, fault(404, "not_found"))
			return
		}
		if err != nil {
			reply(w, nil, err)
			return
		}
		data, err := s.readAsset(item)
		if err != nil {
			reply(w, nil, err)
			return
		}
		w.Header().Set("Content-Type", item.MediaType)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Disposition", "inline")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		// Do not expose local filenames, object keys, validators, or cached 304 paths.
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	})
}
