// Package componentlib defines focal-point and flow-diagram component types.
package componentlib

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	KindFocalPoint  = "focal_point"
	KindFlowDiagram = "flow_diagram"
)

// Category groups components in the palette sidebar.
// org_id NULL = system category; non-null = org-defined custom category.
type Category struct {
	ID        string    `json:"id"`
	OrgID     *string   `json:"orgId,omitempty"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	SortOrder int       `json:"sortOrder"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ComponentField is a configurable field on a component.
type ComponentField struct {
	ID          string   `json:"id"`
	ComponentID string   `json:"componentId"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Readonly    *bool    `json:"readonly,omitempty"`
	Options     []string `json:"options,omitempty"`
	Order       int      `json:"order"`
}

// Component is a catalog entry (built-in or future org-custom).
type Component struct {
	ID           string           `json:"id"`
	OrgID        *string          `json:"orgId,omitempty"`
	Kind         string           `json:"kind"`
	Type         string           `json:"type"`
	Name         string           `json:"name"`
	Slug         string           `json:"slug"`
	Description  string           `json:"description"`
	CategoryID   string           `json:"categoryId"`
	CategoryName string           `json:"categoryName"`
	Tags         []string         `json:"tags"`
	IconKey      *string          `json:"iconKey,omitempty"`
	IsActive     bool             `json:"isActive"`
	Order        int              `json:"order"`
	Fields       []ComponentField `json:"fields,omitempty"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

// FocalPointComponent is the API shape for focal-point palette items.
type FocalPointComponent struct {
	ComponentID     string            `json:"componentId"`
	Type            string            `json:"type"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Category        string            `json:"category"`
	Tags            []string          `json:"tags"`
	Slug            string            `json:"slug"`
	PreviewImageJpg string            `json:"previewImageJpg"`
	IsActive        bool              `json:"isActive"`
	Order           int               `json:"order"`
	ComponentFields []FocalPointField `json:"componentFields"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// FocalPointField mirrors enterprise component field JSON.
type FocalPointField struct {
	ComponentFieldID string   `json:"componentFieldId"`
	Label            string   `json:"label"`
	Type             string   `json:"type"`
	Required         bool     `json:"required"`
	Readonly         *bool    `json:"readonly,omitempty"`
	Options          []string `json:"options,omitempty"`
	Order            int      `json:"order"`
}

// FlowDiagramComponent is the API shape for flow-diagram palette items.
type FlowDiagramComponent struct {
	ComponentID                string             `json:"componentId"`
	Type                       string             `json:"type"`
	Name                       string             `json:"name"`
	Description                string             `json:"description"`
	Category                   string             `json:"category"`
	Tags                       []string           `json:"tags"`
	Slug                       string             `json:"slug"`
	PreviewImageJpg            string             `json:"previewImageJpg"`
	IsActive                   bool               `json:"isActive"`
	Order                      int                `json:"order"`
	OrganizationID             *string            `json:"organizationId,omitempty"`
	FlowDiagramComponentFields []FlowDiagramField `json:"flowDiagramComponentFields"`
}

// FlowDiagramField mirrors enterprise flow diagram field JSON.
type FlowDiagramField struct {
	FlowDiagramComponentFieldID string   `json:"flowDiagramComponentFieldId"`
	Label                       string   `json:"label"`
	Type                        string   `json:"type"`
	Required                    bool     `json:"required"`
	Readonly                    *bool    `json:"readonly,omitempty"`
	Options                     []string `json:"options,omitempty"`
	Order                       int      `json:"order"`
}

// Store is the persistence interface for the component catalog.
type Store interface {
	UpsertComponentCategory(ctx context.Context, cat Category) error
	UpsertComponent(ctx context.Context, c Component) error
	UpsertComponentField(ctx context.Context, f ComponentField) error
	ListComponentsByKind(ctx context.Context, kind string) ([]Component, error)
	UpdateComponentIconKey(ctx context.Context, id, iconKey string) error
	CountComponents(ctx context.Context) (int, error)

	ListCustomComponents(ctx context.Context, orgID string) ([]Component, error)
	GetComponent(ctx context.Context, id string) (*Component, error)
	SaveCustomComponent(ctx context.Context, c Component) error
	DeleteComponent(ctx context.Context, id string) error
}

// CategoryID returns a stable ID for a system category.
func CategoryID(kind, name string) string {
	return fmt.Sprintf("category_%s_%s", kind, Slugify(name))
}

// TagsJSON marshals tags for Postgres JSONB columns.
func TagsJSON(tags []string) json.RawMessage {
	if tags == nil {
		tags = []string{}
	}
	b, _ := json.Marshal(tags)
	return b
}

// OptionsJSON marshals options for Postgres JSONB columns.
// Returns nil for SQL NULL when there are no options (empty json.RawMessage is invalid JSON).
func OptionsJSON(opts []string) any {
	if len(opts) == 0 {
		return nil
	}
	b, _ := json.Marshal(opts)
	return b
}

// ExtractCategories returns deduplicated categories from a component manifest.
func ExtractCategories(components []Component) []Category {
	seen := map[string]bool{}
	order := map[string]int{}
	var cats []Category

	for _, c := range components {
		id := c.CategoryID
		if !seen[id] {
			seen[id] = true
			order[id] = c.Order
			cats = append(cats, Category{
				ID:        id,
				Kind:      c.Kind,
				Name:      c.CategoryName,
				Slug:      Slugify(c.CategoryName),
				SortOrder: c.Order,
				IsActive:  true,
			})
			continue
		}
		if c.Order < order[id] {
			order[id] = c.Order
			for i := range cats {
				if cats[i].ID == id {
					cats[i].SortOrder = c.Order
					break
				}
			}
		}
	}
	return cats
}
