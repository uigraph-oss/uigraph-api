// Package storage provides a thin interface over object storage backends
// (MinIO, S3). The diagram content and file uploads land here.
package storage

import (
	"context"
	"io"

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
	// PresignURL returns a short-lived (15 min) GET URL for key.
	PresignURL(ctx context.Context, key string) (string, error)
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

// FileKey returns the object key for a user-uploaded file.
func FileKey(orgID, fileID, filename string) string {
	return orgID + "/files/" + fileID + "/" + filename
}

func AssetKey(assetID string) string {
	return "assets/" + assetID
}

func NewFileAssetID() string {
	return "file_" + uuid.NewString()
}

func DiagramThumbnailAssetID(diagramID string) string {
	return "diagram_" + diagramID
}

func FrameScreenshotAssetID(frameID string) string {
	return "frame_" + frameID
}

// ComponentIconKey returns the object key for a native component icon SVG.
func ComponentIconKey(slug string) string {
	return "system/components/icons/" + slug + ".svg"
}
