package store

import (
	"errors"

	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/comment"
	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/docs"
	"github.com/uigraph/app/internal/folder"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/llm"
	"github.com/uigraph/app/internal/mcpusage"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/uimap"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrTeamNotFound = errors.New("team not found")
)

type Store interface {
	authz.RBACStore
	identity.SessionStore
	identity.ProviderStore
	identity.ServiceAccountStore
	identity.FigmaAuthStore
	org.UserStore
	org.OrgStore
	org.MemberStore
	org.TeamStore
	folder.Store
	diagram.Store
	docs.Store
	uimap.Store
	catalog.Store
	componentlib.Store
	comment.Store
	llm.Store
	mcpusage.Store
}
