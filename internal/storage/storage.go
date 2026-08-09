// Package storage provides a thin interface over object storage backends
// (MinIO, S3). The diagram content and file uploads land here.
package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// Client is the object storage interface used by handlers.
type Client interface {
	// EnsureBucket creates the configured bucket if it does not already exist.
	// Called once at server startup.
	EnsureBucket(ctx context.Context) error
	// Upload stores r under key in the configured bucket.
	// size may be -1 if unknown (causes buffered upload).
	Upload(ctx context.Context, key, contentType string, r io.Reader, size int64) error
	// Download fetches key. Caller must close the returned ReadCloser.
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes key. Not found is not an error.
	Delete(ctx context.Context, key string) error
	// PresignURL returns a presigned GET URL for key, valid for ttl.
	PresignURL(ctx context.Context, key string, ttl time.Duration) (string, error)
	// PresignPutURL returns a short-lived (15 min) PUT URL for uploading to key.
	PresignPutURL(ctx context.Context, key string) (string, error)
}

// DiagramContentKey returns the object key for a diagram's current content.
func DiagramContentKey(orgID, diagramID string) string {
	return orgID + "/diagrams/" + diagramID + "/content.json"
}

// DiagramVersionKey returns the object key for a specific version snapshot.
func DiagramVersionKey(orgID, diagramID, versionID string) string {
	return orgID + "/diagrams/" + diagramID + "/versions/" + versionID + ".json"
}

// APIGroupSpecKey returns the object key for an API group's spec file.
func APIGroupSpecKey(orgID, serviceID, apiGroupID string) string {
	return orgID + "/services/" + serviceID + "/api-groups/" + apiGroupID + "/spec"
}

// APIGroupVersionSpecKey returns the object key for a version snapshot of an API group spec.
func APIGroupVersionSpecKey(orgID, serviceID, apiGroupID, versionID string) string {
	return orgID + "/services/" + serviceID + "/api-groups/" + apiGroupID + "/versions/" + versionID + "/spec"
}

// ServiceDocFileKey returns the object key for a service doc file.
func ServiceDocFileKey(orgID, serviceID, docID, filename string) string {
	return orgID + "/services/" + serviceID + "/docs/" + docID + "/" + filename
}

// MLArtifactKey returns the object key for a manually uploaded run artifact.
func MLArtifactKey(orgID, runID, artifactID, filename string) string {
	return orgID + "/ml/runs/" + runID + "/artifacts/" + artifactID + "/" + filename
}

// FileKey returns the object key for a user-uploaded file.
func FileKey(orgID, fileID, filename string) string {
	return orgID + "/files/" + fileID + "/" + filename
}

func AssetKey(assetID string) string {
	return "assets/" + assetID
}

// Downloader is the minimal storage surface needed to read an object back.
type Downloader interface {
	Download(ctx context.Context, key string) (io.ReadCloser, error)
}

// HashAsset streams the object stored under assetID and returns its sha256 hex.
func HashAsset(ctx context.Context, c Downloader, assetID string) (string, error) {
	rc, err := c.Download(ctx, AssetKey(assetID))
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, rc); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func NewFileAssetID() string {
	return "file_" + uuid.NewString()
}

// FileAssetIDFromHash derives a deterministic file asset ID from the sha256 hex
// of the content it will hold. Callers that re-upload byte-identical content
// (the CLI on every CI run) then overwrite the same object instead of minting a
// fresh ID and orphaning the previous one.
func FileAssetIDFromHash(contentHash string) string {
	return "file_" + contentHash
}

// IsSHA256Hex reports whether s is a 64-character lowercase hex string, the
// form FileAssetIDFromHash expects.
func IsSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if r >= 'a' && r <= 'f' {
			continue
		}
		return false
	}
	return true
}

func DiagramThumbnailAssetID(diagramID string) string {
	return "diagram_" + diagramID
}

func FrameScreenshotAssetID(frameID string) string {
	return "frame_" + frameID
}

func UserAvatarAssetID(userID string) string {
	return "user_" + userID
}

func ServiceAccountAvatarAssetID(saID string) string {
	return "sa_" + saID
}

func OrgLogoAssetID(orgID string) string {
	return "org_" + orgID
}

func AuthProviderIconAssetID(providerID string) string {
	return "authprovider_" + providerID
}

// ComponentIconKey returns the object key for a native component icon SVG.
func ComponentIconKey(slug string) string {
	return "system/components/icons/" + slug + ".svg"
}
