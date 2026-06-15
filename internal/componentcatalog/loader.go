package componentcatalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed focal_point_components.json
var focalPointRaw []byte

//go:embed flow_diagram_components.json
var flowDiagramRaw []byte

type focalPointFile struct {
	Components []focalPointEntry `json:"components"`
}

type focalPointEntry struct {
	Name            string              `json:"name"`
	Type            string              `json:"type"`
	Description     string              `json:"description"`
	Category        string              `json:"category"`
	Tags            []string            `json:"tags"`
	IsActive        bool                `json:"isActive"`
	Order           int                 `json:"order"`
	ComponentFields []focalPointFieldIn `json:"componentFields"`
}

type focalPointFieldIn struct {
	ComponentFieldID string   `json:"componentFieldId"`
	Label            string   `json:"label"`
	Type             string   `json:"type"`
	Required         bool     `json:"required"`
	Readonly         *bool    `json:"readonly,omitempty"`
	Options          []string `json:"options,omitempty"`
	Order            int      `json:"order"`
}

type flowDiagramFile struct {
	FlowDiagramComponents []flowDiagramEntry `json:"flowDiagramComponents"`
}

type flowDiagramEntry struct {
	Name                       string               `json:"name"`
	Type                       string               `json:"type"`
	Description                string               `json:"description"`
	Category                   string               `json:"category"`
	Tags                       []string             `json:"tags"`
	IsActive                   bool                 `json:"isActive"`
	Order                      int                  `json:"order"`
	FlowDiagramComponentFields []flowDiagramFieldIn `json:"flowDiagramComponentFields"`
}

type flowDiagramFieldIn struct {
	FlowDiagramComponentFieldID string   `json:"flowDiagramComponentFieldId"`
	Label                       string   `json:"label"`
	Type                        string   `json:"type"`
	Required                    bool     `json:"required"`
	Readonly                    *bool    `json:"readonly,omitempty"`
	Options                     []string `json:"options,omitempty"`
	Order                       int      `json:"order"`
}

// LoadManifest parses embedded JSON and returns all native components to seed.
func LoadManifest() ([]Component, error) {
	var out []Component

	focal, err := parseFocalPointFile()
	if err != nil {
		return nil, err
	}
	out = append(out, focal...)

	flow, err := parseFlowDiagramFile()
	if err != nil {
		return nil, err
	}
	out = append(out, flow...)

	return out, nil
}

func parseFocalPointFile() ([]Component, error) {
	var parsed focalPointFile
	if err := json.Unmarshal(focalPointRaw, &parsed); err != nil {
		return nil, fmt.Errorf("componentcatalog: parse focal point json: %w", err)
	}
	out := make([]Component, 0, len(parsed.Components))
	for _, e := range parsed.Components {
		nameSlug := Slugify(e.Name)
		id := "component_" + nameSlug
		c := Component{
			ID:           id,
			Kind:         KindFocalPoint,
			Type:         e.Type,
			Name:         e.Name,
			Slug:         id,
			Description:  e.Description,
			CategoryID:   CategoryID(KindFocalPoint, e.Category),
			CategoryName: e.Category,
			Tags:        e.Tags,
			IsActive:    e.IsActive,
			Order:       e.Order,
		}
		for _, f := range e.ComponentFields {
			c.Fields = append(c.Fields, ComponentField{
				ID:          f.ComponentFieldID,
				ComponentID: id,
				Label:       f.Label,
				Type:        f.Type,
				Required:    f.Required,
				Readonly:    f.Readonly,
				Options:     f.Options,
				Order:       f.Order,
			})
		}
		out = append(out, c)
	}
	return out, nil
}

func parseFlowDiagramFile() ([]Component, error) {
	var parsed flowDiagramFile
	if err := json.Unmarshal(flowDiagramRaw, &parsed); err != nil {
		return nil, fmt.Errorf("componentcatalog: parse flow diagram json: %w", err)
	}
	out := make([]Component, 0, len(parsed.FlowDiagramComponents))
	for _, e := range parsed.FlowDiagramComponents {
		nameSlug := Slugify(e.Name)
		id := "flow_diagram_component_" + nameSlug
		c := Component{
			ID:           id,
			Kind:         KindFlowDiagram,
			Type:         e.Type,
			Name:         e.Name,
			Slug:         nameSlug,
			Description:  e.Description,
			CategoryID:   CategoryID(KindFlowDiagram, e.Category),
			CategoryName: e.Category,
			Tags:        e.Tags,
			IsActive:    e.IsActive,
			Order:       e.Order,
		}
		for _, f := range e.FlowDiagramComponentFields {
			c.Fields = append(c.Fields, ComponentField{
				ID:          f.FlowDiagramComponentFieldID,
				ComponentID: id,
				Label:       f.Label,
				Type:        f.Type,
				Required:    f.Required,
				Readonly:    f.Readonly,
				Options:     f.Options,
				Order:       f.Order,
			})
		}
		out = append(out, c)
	}
	return out, nil
}

// Slugify converts a component name into a URL-friendly slug (matches enterprise slug.Make).
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// IconSlug returns the object slug used for icon storage (name-based, without prefix).
func IconSlug(c Component) string {
	if c.Kind == KindFlowDiagram {
		return c.Slug
	}
	return strings.TrimPrefix(c.ID, "component_")
}
