// Package store defines the aggregate Store interface for the UIGraph application.
// Domain types and sub-store interfaces live in their own packages (authz, identity, org).
// The postgres implementation lives in store/postgres.
package store

import (
	"errors"

	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/comment"
	"github.com/uigraph/app/internal/componentcatalog"
	"github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/folder"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/uimap"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

// Store is the single persistence interface passed to the entire application.
// It composes every domain store interface; the postgres.DB implements all of them.
type Store interface {
	authz.RBACStore
	identity.SessionStore
	identity.ProviderStore
	identity.ServiceAccountStore
	org.UserStore
	org.OrgStore
	org.MemberStore
	org.TeamStore
	org.InvitationStore
	folder.Store
	diagram.Store
	uimap.Store
	catalog.Store
	componentcatalog.Store
	comment.Store
}
