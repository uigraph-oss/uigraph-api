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

type FlowComponentHandler struct{}

func NewFlowComponentHandler() *FlowComponentHandler { return &FlowComponentHandler{} }

func (h *FlowComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"components":       loadFlowCatalog(),
		"customComponents": []FlowDiagramComponent{},
	})
}

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
