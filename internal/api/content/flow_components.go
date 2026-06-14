package content

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

//go:embed flow_diagram_components.json
var flowComponentCatalogRaw []byte

// FlowDiagramComponentField mirrors a single configurable field on a component.
type FlowDiagramComponentField struct {
	FlowDiagramComponentFieldID string   `json:"flowDiagramComponentFieldId"`
	Label                       string   `json:"label"`
	Type                        string   `json:"type"`
	Required                    bool     `json:"required"`
	Readonly                    *bool    `json:"readonly,omitempty"`
	Options                     []string `json:"options,omitempty"`
	Order                       int      `json:"order"`
}

// FlowDiagramComponent is a single palette component (native or custom).
type FlowDiagramComponent struct {
	ComponentID                string                      `json:"componentId"`
	Type                       string                      `json:"type"`
	Name                       string                      `json:"name"`
	Description                string                      `json:"description"`
	Category                   string                      `json:"category"`
	Tags                       []string                    `json:"tags"`
	Slug                       string                      `json:"slug"`
	PreviewImageJpg            string                      `json:"previewImageJpg"`
	IsActive                   bool                        `json:"isActive"`
	Order                      int                         `json:"order"`
	OrganizationID             *string                     `json:"organizationId,omitempty"`
	FlowDiagramComponentFields []FlowDiagramComponentField `json:"flowDiagramComponentFields"`
}

var (
	flowCatalogOnce sync.Once
	flowCatalog     []FlowDiagramComponent
)

// loadFlowCatalog parses the embedded native component catalog once, deriving a
// stable componentId and slug for each entry.
func loadFlowCatalog() []FlowDiagramComponent {
	flowCatalogOnce.Do(func() {
		var parsed struct {
			FlowDiagramComponents []FlowDiagramComponent `json:"flowDiagramComponents"`
		}
		if err := json.Unmarshal(flowComponentCatalogRaw, &parsed); err != nil {
			flowCatalog = []FlowDiagramComponent{}
			return
		}
		for i := range parsed.FlowDiagramComponents {
			c := &parsed.FlowDiagramComponents[i]
			c.Slug = slugify(c.Name)
			c.ComponentID = "flow_diagram_component_" + c.Slug
		}
		flowCatalog = parsed.FlowDiagramComponents
	})
	return flowCatalog
}

// FlowComponentHandler serves the flow diagram component palette.
type FlowComponentHandler struct{}

func NewFlowComponentHandler() *FlowComponentHandler { return &FlowComponentHandler{} }

// List handles GET /api/v1/orgs/{orgID}/flow-diagram-components
// Native components come from the embedded catalog; custom components are
// per-org and currently empty (no custom-component authoring in this scope).
func (h *FlowComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"components":       loadFlowCatalog(),
		"customComponents": []FlowDiagramComponent{},
	})
}

// slugify converts a component name into a URL-friendly slug.
func slugify(s string) string {
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
