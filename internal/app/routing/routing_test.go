package routing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/container"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/middleware"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/auth"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routeStore struct {
	statuses []store.StatusRecord
	err      error
}

func (r *routeStore) WriteStatusEvent(context.Context, store.StatusEvent) error { return nil }
func (r *routeStore) WriteWorkEvent(context.Context, store.WorkEvent) error     { return nil }
func (r *routeStore) ListByServiceOrder(context.Context, string) ([]store.StatusRecord, []store.WorkRecord, error) {
	return r.statuses, nil, r.err
}
func (r *routeStore) ListReport(context.Context, []string) ([]store.ReportRecord, error) {
	return []store.ReportRecord{{WorkID: "w1", WorkName: "Troca de óleo", Count: 1, TotalSeconds: 5400}}, r.err
}

func newRouter(st store.Store) http.Handler {
	c := &container.HandlersWrapper{HistoryHandler: history.NewController(history.NewService(st))}
	m := &container.Middlewares{"Recovery": middleware.Recovery()}
	return SetupRouter(c, m)
}

func tokenFor(t *testing.T, roles ...auth.Role) string {
	t.Helper()
	tok, err := auth.GenerateToken("user-1", roles, time.Now().Add(time.Hour))
	require.NoError(t, err)
	return "Bearer " + tok
}

func do(t *testing.T, h http.Handler, method, path, authz string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHistoryRouteRequiresAuthAndRole(t *testing.T) {
	h := newRouter(&routeStore{})
	path := "/v1/service-order/so-1/history"

	assert.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, path, "").Code)
	assert.Equal(t, http.StatusForbidden, do(t, h, http.MethodGet, path, tokenFor(t, auth.CUSTOMER)).Code)
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, path, tokenFor(t, auth.ATTENDANT)).Code)
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, path, tokenFor(t, auth.MECHANIC)).Code)
}

func TestHistoryRouteReturnsJSONArray(t *testing.T) {
	st := &routeStore{statuses: []store.StatusRecord{{NewStatus: "NEW", CreatedAt: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)}}}
	rec := do(t, newRouter(st), http.MethodGet, "/v1/service-order/so-1/history", tokenFor(t, auth.ATTENDANT))

	require.Equal(t, http.StatusOK, rec.Code)
	var body []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 1)
	assert.Equal(t, "NEW", body[0]["newStatus"])
	assert.Equal(t, "", body[0]["previousStatus"])
}

func TestHistoryRouteEmptyIsArray(t *testing.T) {
	rec := do(t, newRouter(&routeStore{}), http.MethodGet, "/v1/service-order/so-1/history", tokenFor(t, auth.ATTENDANT))
	assert.Equal(t, "[]", rec.Body.String())
}

func TestHistoryRouteStoreErrorIsInternalError(t *testing.T) {
	rec := do(t, newRouter(&routeStore{err: errors.New("dynamo down")}), http.MethodGet, "/v1/service-order/so-1/history", tokenFor(t, auth.ATTENDANT))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "dynamo down")
}

func TestReportRouteAllowsOnlyAttendantAndAdmin(t *testing.T) {
	h := newRouter(&routeStore{})
	path := "/v1/reports/average-execution-time?work_id=w1"

	assert.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, path, "").Code)
	assert.Equal(t, http.StatusForbidden, do(t, h, http.MethodGet, path, tokenFor(t, auth.MECHANIC)).Code)

	rec := do(t, h, http.MethodGet, path, tokenFor(t, auth.ADMIN))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"workId":"w1","workName":"Troca de óleo","averageExecutionTimeHours":1.5}]`, rec.Body.String())
}

func TestOperationalRoutes(t *testing.T) {
	h := newRouter(&routeStore{})
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, "/ping", "").Code)
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, "/metrics", "").Code)
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, "/swagger.yaml", "").Code)
}
