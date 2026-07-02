package catalog

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// ListTestPacks
// @Summary  ListTestPacks
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-packs [get]
func (h *Handler) ListTestPacks(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	packs, err := h.store.ListTestPacks(r.Context(), serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"testPacks": packs})
}

// CreateTestPack
// @Summary  CreateTestPack
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-pack [post]
func (h *Handler) CreateTestPack(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.Type == "" {
		body.Type = "manual"
	}
	now := time.Now().UTC()
	pack := catalogpkg.TestPack{
		ID:        uuid.NewString(),
		ServiceID: serviceID,
		OrgID:     orgID,
		Name:      body.Name,
		Type:      body.Type,
		CreatedBy: p.UserID,
		UpdatedBy: &p.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.store.CreateTestPack(r.Context(), pack); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, pack)
}

// UpdateTestPack
// @Summary  UpdateTestPack
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    testPackID  path  string  true  "testPackID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-pack/{testPackID} [post]
func (h *Handler) UpdateTestPack(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testPackID := r.PathValue("testPackID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	pack, err := h.store.GetTestPack(r.Context(), testPackID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if pack == nil || pack.DeletedAt != nil || pack.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		Name *string `json:"name"`
		Type *string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		pack.Name = *body.Name
	}
	if body.Type != nil {
		pack.Type = *body.Type
	}
	pack.UpdatedBy = &p.UserID
	if err := h.store.UpdateTestPack(r.Context(), *pack); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, pack)
}

// DeleteTestPack
// @Summary  DeleteTestPack
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    testPackID  path  string  true  "testPackID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-pack/{testPackID} [delete]
func (h *Handler) DeleteTestPack(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testPackID := r.PathValue("testPackID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	pack, err := h.store.GetTestPack(r.Context(), testPackID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if pack == nil || pack.DeletedAt != nil || pack.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.SoftDeleteTestPack(r.Context(), testPackID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTestCases
// @Summary  ListTestCases
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-cases [get]
func (h *Handler) ListTestCases(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	var testPackID *string
	if v := r.URL.Query().Get("testPackId"); v != "" {
		testPackID = &v
	}
	cases, err := h.store.ListTestCases(r.Context(), serviceID, testPackID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"testCases": cases})
}

// CreateTestCase
// @Summary  CreateTestCase
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-case [post]
func (h *Handler) CreateTestCase(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var tc catalogpkg.TestCase
	if err := json.NewDecoder(r.Body).Decode(&tc); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if tc.TestPackID == "" || tc.Title == "" || tc.Type == "" {
		httputil.BadRequest(w, "testPackId, title and type are required")
		return
	}
	pack, err := h.store.GetTestPack(r.Context(), tc.TestPackID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if pack == nil || pack.DeletedAt != nil || pack.ServiceID != serviceID {
		httputil.BadRequest(w, "invalid testPackId")
		return
	}
	now := time.Now().UTC()
	tc.ID = uuid.NewString()
	tc.ServiceID = serviceID
	tc.OrgID = orgID
	tc.CreatedBy = p.UserID
	tc.UpdatedBy = &p.UserID
	tc.CreatedAt = now
	tc.UpdatedAt = now
	if tc.Status == "" {
		tc.Status = "active"
	}
	if tc.Version == 0 {
		tc.Version = 1
	}
	if err := h.store.CreateTestCase(r.Context(), tc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, tc)
}

// UpdateTestCase
// @Summary  UpdateTestCase
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    testCaseID  path  string  true  "testCaseID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-case/{testCaseID} [post]
func (h *Handler) UpdateTestCase(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testCaseID := r.PathValue("testCaseID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	tc, err := h.store.GetTestCase(r.Context(), testCaseID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if tc == nil || tc.DeletedAt != nil || tc.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body catalogpkg.TestCase
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.TestPackID != "" {
		tc.TestPackID = body.TestPackID
	}
	if body.Title != "" {
		tc.Title = body.Title
	}
	if body.Type != "" {
		tc.Type = body.Type
	}
	if body.Order != 0 {
		tc.Order = body.Order
	}
	if body.Description != nil {
		tc.Description = body.Description
	}
	if body.Priority != nil {
		tc.Priority = body.Priority
	}
	if body.Labels != nil {
		tc.Labels = body.Labels
	}
	if body.LinkedTicket != nil {
		tc.LinkedTicket = body.LinkedTicket
	}
	if body.EstimatedDurationMins != nil {
		tc.EstimatedDurationMins = body.EstimatedDurationMins
	}
	if body.TestOwner != nil {
		tc.TestOwner = body.TestOwner
	}
	if body.LinkedMapNodeID != nil {
		tc.LinkedMapNodeID = body.LinkedMapNodeID
	}
	tc.IsCritical = body.IsCritical
	tc.EvidenceRequired = body.EvidenceRequired
	if body.Manual != nil {
		tc.Manual = body.Manual
	}
	if body.API != nil {
		tc.API = body.API
	}
	if body.GraphQL != nil {
		tc.GraphQL = body.GraphQL
	}
	if body.Database != nil {
		tc.Database = body.Database
	}
	if body.GRPC != nil {
		tc.GRPC = body.GRPC
	}
	if body.Status != "" {
		tc.Status = body.Status
	}
	if body.Version > 0 {
		tc.Version = body.Version
	}
	if body.BaselineRunResultID != nil {
		tc.BaselineRunResultID = body.BaselineRunResultID
	}
	if body.Dependencies != nil {
		tc.Dependencies = body.Dependencies
	}
	tc.UpdatedBy = &p.UserID
	if err := h.store.UpdateTestCase(r.Context(), *tc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, tc)
}

// DeleteTestCase
// @Summary  DeleteTestCase
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    testCaseID  path  string  true  "testCaseID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-case/{testCaseID} [delete]
func (h *Handler) DeleteTestCase(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testCaseID := r.PathValue("testCaseID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	tc, err := h.store.GetTestCase(r.Context(), testCaseID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if tc == nil || tc.DeletedAt != nil || tc.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.SoftDeleteTestCase(r.Context(), testCaseID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateTestRun
// @Summary  CreateTestRun
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-run [post]
func (h *Handler) CreateTestRun(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var tr catalogpkg.TestRun
	if err := json.NewDecoder(r.Body).Decode(&tr); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if tr.TestPackID == "" || tr.Environment == "" {
		httputil.BadRequest(w, "testPackId and environment are required")
		return
	}
	now := time.Now().UTC()
	tr.ID = uuid.NewString()
	tr.ServiceID = serviceID
	tr.OrgID = orgID
	tr.CreatedBy = p.UserID
	tr.UpdatedBy = &p.UserID
	tr.ExecutedBy = p.UserID
	tr.ExecutedAt = now
	tr.CreatedAt = now
	tr.UpdatedAt = now
	if tr.Status == "" {
		tr.Status = "running"
	}
	if tr.OverallStatus == "" {
		tr.OverallStatus = "partial"
	}
	if err := h.store.CreateTestRun(r.Context(), tr); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, tr)
}

// GetTestRun
// @Summary  GetTestRun
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    testRunID  path  string  true  "testRunID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-run/{testRunID} [get]
func (h *Handler) GetTestRun(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testRunID := r.PathValue("testRunID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	tr, err := h.store.GetTestRun(r.Context(), testRunID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if tr == nil || tr.DeletedAt != nil || tr.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, tr)
}

// ListTestRuns
// @Summary  ListTestRuns
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-runs [get]
func (h *Handler) ListTestRuns(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	var testPackID *string
	if v := r.URL.Query().Get("testPackId"); v != "" {
		testPackID = &v
	}
	runs, err := h.store.ListTestRuns(r.Context(), serviceID, testPackID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"testRuns": runs})
}

// ListTestRunsSummary
// @Summary  ListTestRunsSummary
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-runs-summary [get]
func (h *Handler) ListTestRunsSummary(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	q := r.URL.Query()
	filter := catalogpkg.TestRunSummaryFilter{}
	if v := q.Get("testPackId"); v != "" {
		filter.TestPackID = &v
	}
	if v := q.Get("environment"); v != "" {
		filter.Environment = &v
	}
	if v := q.Get("status"); v != "" {
		filter.Status = &v
	}
	if v := q.Get("executedBy"); v != "" {
		filter.ExecutedBy = &v
	}
	if v := q.Get("fromDate"); v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			filter.FromDate = &ts
		}
	}
	if v := q.Get("toDate"); v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			filter.ToDate = &ts
		}
	}
	summary, err := h.store.ListTestRunsSummary(r.Context(), serviceID, filter)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"testRunsSummary": summary})
}

// UpdateTestRun
// @Summary  UpdateTestRun
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    testRunID  path  string  true  "testRunID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-run/{testRunID} [post]
func (h *Handler) UpdateTestRun(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testRunID := r.PathValue("testRunID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	tr, err := h.store.GetTestRun(r.Context(), testRunID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if tr == nil || tr.DeletedAt != nil || tr.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		OverallStatus *string `json:"overallStatus"`
		CompletedAt   *string `json:"completedAt"`
		Status        *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.OverallStatus != nil {
		tr.OverallStatus = *body.OverallStatus
	}
	if body.Status != nil {
		tr.Status = *body.Status
	}
	if body.CompletedAt != nil && *body.CompletedAt != "" {
		if ts, err := time.Parse(time.RFC3339, *body.CompletedAt); err == nil {
			tr.CompletedAt = &ts
		}
	}
	tr.UpdatedBy = &p.UserID
	if err := h.store.UpdateTestRun(r.Context(), *tr); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, tr)
}

// CreateTestRunResult
// @Summary  CreateTestRunResult
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-run-result [post]
func (h *Handler) CreateTestRunResult(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var rr catalogpkg.TestRunResult
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if rr.TestRunID == "" || rr.TestCaseID == "" || rr.Status == "" {
		httputil.BadRequest(w, "testRunId, testCaseId and status are required")
		return
	}
	now := time.Now().UTC()
	rr.ID = uuid.NewString()
	rr.ServiceID = serviceID
	rr.OrgID = orgID
	rr.ExecutedBy = p.UserID
	rr.ExecutedAt = now
	rr.CreatedAt = now
	rr.UpdatedAt = now
	if err := h.store.CreateTestRunResult(r.Context(), rr); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, rr)
}

// ListTestRunResults
// @Summary  ListTestRunResults
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-run-results [get]
func (h *Handler) ListTestRunResults(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	testRunID := r.URL.Query().Get("testRunId")
	if testRunID == "" {
		httputil.BadRequest(w, "testRunId is required")
		return
	}
	results, err := h.store.ListTestRunResults(r.Context(), serviceID, testRunID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"testRunResults": results})
}

// UpdateTestRunResult
// @Summary  UpdateTestRunResult
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    testRunResultID  path  string  true  "testRunResultID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/test-run-result/{testRunResultID} [post]
func (h *Handler) UpdateTestRunResult(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	resultID := r.PathValue("testRunResultID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	rr, err := h.store.GetTestRunResult(r.Context(), resultID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rr == nil || rr.DeletedAt != nil || rr.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body catalogpkg.TestRunResult
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Status != "" {
		rr.Status = body.Status
	}
	if body.BlockedReason != nil {
		rr.BlockedReason = body.BlockedReason
	}
	if body.ResponseStatus != nil {
		rr.ResponseStatus = body.ResponseStatus
	}
	if body.ResponseBody != nil {
		rr.ResponseBody = body.ResponseBody
	}
	if body.ResponseTimeMs != nil {
		rr.ResponseTimeMs = body.ResponseTimeMs
	}
	if body.Notes != nil {
		rr.Notes = body.Notes
	}
	if body.ScreenshotURLs != nil {
		rr.ScreenshotURLs = body.ScreenshotURLs
	}
	rr.ExecutedBy = p.UserID
	rr.ExecutedAt = time.Now().UTC()
	if err := h.store.UpdateTestRunResult(r.Context(), *rr); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, rr)
}
