package content

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/catalog"
	authmw "github.com/uigraph/app/internal/middleware"
)

func (h *ServiceHandler) ListTestPacks(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	packs, err := h.store.ListTestPacks(r.Context(), serviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"testPacks": packs})
}

func (h *ServiceHandler) CreateTestPack(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Type == "" {
		body.Type = "manual"
	}
	now := time.Now().UTC()
	pack := catalog.TestPack{
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, pack)
}

func (h *ServiceHandler) UpdateTestPack(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testPackID := r.PathValue("testPackID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	pack, err := h.store.GetTestPack(r.Context(), testPackID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if pack == nil || pack.DeletedAt != nil || pack.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var body struct {
		Name *string `json:"name"`
		Type *string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

func (h *ServiceHandler) DeleteTestPack(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testPackID := r.PathValue("testPackID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	pack, err := h.store.GetTestPack(r.Context(), testPackID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if pack == nil || pack.DeletedAt != nil || pack.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := h.store.SoftDeleteTestPack(r.Context(), testPackID, p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ServiceHandler) ListTestCases(w http.ResponseWriter, r *http.Request) {
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"testCases": cases})
}

func (h *ServiceHandler) CreateTestCase(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var tc catalog.TestCase
	if err := json.NewDecoder(r.Body).Decode(&tc); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if tc.TestPackID == "" || tc.Title == "" || tc.Type == "" {
		writeErr(w, http.StatusBadRequest, "testPackId, title and type are required")
		return
	}
	pack, err := h.store.GetTestPack(r.Context(), tc.TestPackID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if pack == nil || pack.DeletedAt != nil || pack.ServiceID != serviceID {
		writeErr(w, http.StatusBadRequest, "invalid testPackId")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, tc)
}

func (h *ServiceHandler) UpdateTestCase(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testCaseID := r.PathValue("testCaseID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	tc, err := h.store.GetTestCase(r.Context(), testCaseID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tc == nil || tc.DeletedAt != nil || tc.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var body catalog.TestCase
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, tc)
}

func (h *ServiceHandler) DeleteTestCase(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testCaseID := r.PathValue("testCaseID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	tc, err := h.store.GetTestCase(r.Context(), testCaseID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tc == nil || tc.DeletedAt != nil || tc.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := h.store.SoftDeleteTestCase(r.Context(), testCaseID, p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ServiceHandler) CreateTestRun(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var tr catalog.TestRun
	if err := json.NewDecoder(r.Body).Decode(&tr); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if tr.TestPackID == "" || tr.Environment == "" {
		writeErr(w, http.StatusBadRequest, "testPackId and environment are required")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, tr)
}

func (h *ServiceHandler) GetTestRun(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testRunID := r.PathValue("testRunID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	tr, err := h.store.GetTestRun(r.Context(), testRunID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tr == nil || tr.DeletedAt != nil || tr.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, tr)
}

func (h *ServiceHandler) ListTestRuns(w http.ResponseWriter, r *http.Request) {
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"testRuns": runs})
}

func (h *ServiceHandler) ListTestRunsSummary(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	q := r.URL.Query()
	filter := catalog.TestRunSummaryFilter{}
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"testRunsSummary": summary})
}

func (h *ServiceHandler) UpdateTestRun(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	testRunID := r.PathValue("testRunID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	tr, err := h.store.GetTestRun(r.Context(), testRunID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tr == nil || tr.DeletedAt != nil || tr.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var body struct {
		OverallStatus *string `json:"overallStatus"`
		CompletedAt   *string `json:"completedAt"`
		Status        *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, tr)
}

func (h *ServiceHandler) CreateTestRunResult(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var rr catalog.TestRunResult
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if rr.TestRunID == "" || rr.TestCaseID == "" || rr.Status == "" {
		writeErr(w, http.StatusBadRequest, "testRunId, testCaseId and status are required")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, rr)
}

func (h *ServiceHandler) ListTestRunResults(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	testRunID := r.URL.Query().Get("testRunId")
	if testRunID == "" {
		writeErr(w, http.StatusBadRequest, "testRunId is required")
		return
	}
	results, err := h.store.ListTestRunResults(r.Context(), serviceID, testRunID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"testRunResults": results})
}

func (h *ServiceHandler) UpdateTestRunResult(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	resultID := r.PathValue("testRunResultID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	rr, err := h.store.GetTestRunResult(r.Context(), resultID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if rr == nil || rr.DeletedAt != nil || rr.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var body catalog.TestRunResult
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, rr)
}
