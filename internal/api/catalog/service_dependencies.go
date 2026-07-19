package catalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) SyncDependencies(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if !h.ensureServiceInOrg(w, r, serviceID) {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var body struct {
		Dependencies []struct {
			Name        string   `json:"name"`
			Service     string   `json:"service"`
			Type        string   `json:"type"`
			Criticality string   `json:"criticality"`
			Description      string   `json:"description"`
			APIGroupName     *string  `json:"apiGroupName"`
			APIEndpointNames []string `json:"apiEndpointNames"`
			DatabaseName     *string  `json:"databaseName"`
		} `json:"dependencies"`
		CommitHash *string `json:"commitHash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	dependencies := make([]catalog.ServiceDependency, 0, len(body.Dependencies))
	names := map[string]bool{}
	for _, input := range body.Dependencies {
		if input.Name == "" || input.Service == "" {
			httputil.BadRequest(w, "dependency name and service are required")
			return
		}
		if names[input.Name] {
			httputil.BadRequest(w, "dependency names must be unique")
			return
		}
		names[input.Name] = true
		if input.Type != "" && input.Type != "http" && input.Type != "graphql" && input.Type != "grpc" && input.Type != "database" {
			httputil.BadRequest(w, "dependency type must be http, graphql, grpc, or database")
			return
		}
		if input.Criticality != "hard" && input.Criticality != "soft" {
			httputil.BadRequest(w, "dependency criticality must be hard or soft")
			return
		}
		endpointNames := map[string]bool{}
		for _, endpointName := range input.APIEndpointNames {
			if endpointName == "" {
				httputil.BadRequest(w, "api endpoint names must not be empty")
				return
			}
			if endpointNames[endpointName] {
				httputil.BadRequest(w, "api endpoint names must be unique within a dependency")
				return
			}
			endpointNames[endpointName] = true
		}
		dependencies = append(dependencies, catalog.ServiceDependency{Name: input.Name, ProviderServiceName: input.Service, Type: input.Type, Criticality: input.Criticality, Description: input.Description, APIGroupName: input.APIGroupName, APIEndpointNames: input.APIEndpointNames, DatabaseName: input.DatabaseName})
	}
	if err := h.store.SyncServiceDependencies(r.Context(), r.PathValue("orgID"), serviceID, p.UserID, body.CommitHash, dependencies); err != nil {
		if errors.Is(err, storepkg.ErrInvalidDependency) {
			httputil.BadRequest(w, err.Error())
			return
		}
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"dependencies": dependencies})
}

func (h *Handler) ListDependencies(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if !h.ensureServiceInOrg(w, r, serviceID) {
		return
	}
	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "all"
	}
	if direction != "all" && direction != "upstream" && direction != "downstream" {
		httputil.BadRequest(w, "direction must be all, upstream, or downstream")
		return
	}
	criticality := r.URL.Query().Get("criticality")
	if criticality != "" && criticality != "hard" && criticality != "soft" {
		httputil.BadRequest(w, "criticality must be hard or soft")
		return
	}
	edges, err := h.store.ListServiceDependencies(r.Context(), r.PathValue("orgID"), serviceID, direction, criticality)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"edges": edges})
}

func (h *Handler) GetServiceDependencyGraph(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if !h.ensureServiceInOrg(w, r, serviceID) {
		return
	}
	graph, err := h.store.DependencyGraph(r.Context(), r.PathValue("orgID"), serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, graph)
}

func (h *Handler) GetDependencyGraph(w http.ResponseWriter, r *http.Request) {
	graph, err := h.store.DependencyGraph(r.Context(), r.PathValue("orgID"), "")
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if r.URL.Query().Get("format") == "mermaid" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(catalog.RenderMermaid(graph)))
		return
	}
	httputil.JSON(w, http.StatusOK, graph)
}

func (h *Handler) GetImpact(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if !h.ensureServiceInOrg(w, r, serviceID) {
		return
	}
	direction := r.URL.Query().Get("direction")
	if direction != "upstream" && direction != "downstream" {
		httputil.BadRequest(w, "direction must be upstream or downstream")
		return
	}
	maxDepth := 10
	if value := r.URL.Query().Get("maxDepth"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			httputil.BadRequest(w, "maxDepth must be between 1 and 100")
			return
		}
		maxDepth = parsed
	}
	graph, err := h.store.Impact(r.Context(), r.PathValue("orgID"), serviceID, direction, maxDepth)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, graph)
}
