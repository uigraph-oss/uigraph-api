package org

import "context"

// OrgStore is the persistence interface for organisations.
type OrgStore interface {
	CreateOrg(ctx context.Context, o Org) error
	GetOrg(ctx context.Context, id string) (*Org, error)
	ListOrgs(ctx context.Context) ([]Org, error)
	CountAllOrgs(ctx context.Context) (int, error)
	UpdateOrg(ctx context.Context, o Org) error
	SetOrgLogo(ctx context.Context, id string, assetID *string) error
	DeleteOrg(ctx context.Context, id string) error
	AnyOrgExists(ctx context.Context) (bool, error)
}
