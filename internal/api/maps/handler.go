// Package maps provides HTTP handlers for maps, frames, focal points, and canvas.
package maps

import (
	"context"
	"io"
	"net/http"

	"github.com/uigraph/app/internal/uimap"
)

type store interface {
	CreateMap(ctx context.Context, m uimap.Map) error
	GetMap(ctx context.Context, id string) (*uimap.Map, error)
	ListMaps(ctx context.Context, orgID string, folderID, teamID *string) ([]uimap.Map, error)
	UpdateMap(ctx context.Context, m uimap.Map) error
	SoftDeleteMap(ctx context.Context, id, deletedBy string) error

	CreateFrame(ctx context.Context, f uimap.Frame) error
	GetFrame(ctx context.Context, id string) (*uimap.Frame, error)
	ListFrames(ctx context.Context, mapID string) ([]uimap.Frame, error)
	UpdateFrame(ctx context.Context, f uimap.Frame) error
	SoftDeleteFrame(ctx context.Context, id, deletedBy string) error

	CreateFocalPoint(ctx context.Context, fp uimap.FocalPoint) error
	GetFocalPoint(ctx context.Context, id string) (*uimap.FocalPoint, error)
	ListFocalPoints(ctx context.Context, frameID string) ([]uimap.FocalPoint, error)
	UpdateFocalPoint(ctx context.Context, fp uimap.FocalPoint) error
	SoftDeleteFocalPoint(ctx context.Context, id, deletedBy string) error

	CreateFrameGroup(ctx context.Context, g uimap.FrameGroup) error
	GetFrameGroup(ctx context.Context, id string) (*uimap.FrameGroup, error)
	ListFrameGroups(ctx context.Context, frameID string) ([]uimap.FrameGroup, error)
	UpdateFrameGroup(ctx context.Context, g uimap.FrameGroup) error
	SoftDeleteFrameGroup(ctx context.Context, id, deletedBy string) error

	CreateFrameLink(ctx context.Context, l uimap.FrameLink) error
	GetFrameLink(ctx context.Context, id string) (*uimap.FrameLink, error)
	ListFrameLinks(ctx context.Context, frameID string) ([]uimap.FrameLink, error)
	UpdateFrameLink(ctx context.Context, l uimap.FrameLink) error
	SoftDeleteFrameLink(ctx context.Context, id, deletedBy string) error

	CreateFocalPointMeta(ctx context.Context, m uimap.FocalPointMeta) error
	GetFocalPointMeta(ctx context.Context, id string) (*uimap.FocalPointMeta, error)
	ListFocalPointMeta(ctx context.Context, focalPointID string) ([]uimap.FocalPointMeta, error)
	UpdateFocalPointMeta(ctx context.Context, m uimap.FocalPointMeta) error
	SoftDeleteFocalPointMeta(ctx context.Context, id, deletedBy string) error

	GetCanvas(ctx context.Context, mapID string) (*uimap.Canvas, error)
	UpsertCanvas(ctx context.Context, c uimap.Canvas) error
}

type objectStore interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
}

// Handler serves map, frame, focal-point, canvas, group, link, and meta endpoints.
type Handler struct {
	store   store
	storage objectStore // may be nil (no screenshot upload in some environments)
}

// New constructs a Handler.
func New(s store, st objectStore) *Handler {
	return &Handler{store: s, storage: st}
}

// Register wires all map-domain routes into mux.
func Register(
	mux *http.ServeMux,
	s store,
	st objectStore,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s, st)

	// Maps
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps", h.ListMaps)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps", h.CreateMap)
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}", h.GetMap)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}", h.UpdateMap)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}", h.DeleteMap)

	// Frames
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", h.ListFrames)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", h.CreateFrame)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/sync", h.SyncFrames)
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/frames/{frameID}", h.GetFrame)
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", h.GetFrame)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", h.UpdateFrame)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", h.DeleteFrame)

	// Focal Points
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", h.ListFocalPoints)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", h.CreateFocalPoint)
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", h.GetFocalPoint)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", h.UpdateFocalPoint)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", h.DeleteFocalPoint)

	// Focal Point Meta
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", h.ListMeta)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", h.CreateMeta)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", h.UpdateMeta)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", h.DeleteMeta)

	// Canvas
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", h.GetCanvas)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", h.UpsertCanvas)

	// Groups
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", h.ListGroups)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", h.CreateGroup)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", h.UpdateGroup)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", h.DeleteGroup)

	// Links
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", h.ListLinks)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", h.CreateLink)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", h.UpdateLink)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", h.DeleteLink)
}
