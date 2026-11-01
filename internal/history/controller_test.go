package history

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerGetHistoryReadsIDFromContext(t *testing.T) {
	st := &fakeStore{statuses: []store.StatusRecord{{NewStatus: "NEW", CreatedAt: t0}}}
	c := NewController(NewService(st))

	ctx := context.WithValue(context.Background(), "id", "so-1") //nolint:staticcheck // same route-param convention as GinHandler
	out, err := c.GetHistory(ctx, &http.Request{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "NEW", out[0].NewStatus)
}

func TestControllerGetHistoryWithoutIDIsRejected(t *testing.T) {
	c := NewController(NewService(&fakeStore{}))
	_, err := c.GetHistory(context.Background(), &http.Request{})
	assert.ErrorIs(t, err, ErrServiceOrderIDRequired)
}

func TestControllerGetHistoryPropagatesStoreError(t *testing.T) {
	c := NewController(NewService(&fakeStore{err: errBoom}))
	ctx := context.WithValue(context.Background(), "id", "so-1") //nolint:staticcheck
	_, err := c.GetHistory(ctx, &http.Request{})
	assert.ErrorIs(t, err, errBoom)
}

func TestControllerReportReadsRepeatedWorkIDs(t *testing.T) {
	st := &fakeStore{reports: nil}
	c := NewController(NewService(st))

	req := &http.Request{URL: &url.URL{RawQuery: "work_id=a&work_id=b"}}
	out, err := c.GetAverageExecutionTime(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, st.gotReportIDs)
	assert.NotNil(t, out)
	assert.Empty(t, out)
}

func TestControllerReportWithoutFilterAsksForAllWorks(t *testing.T) {
	st := &fakeStore{}
	c := NewController(NewService(st))

	_, err := c.GetAverageExecutionTime(context.Background(), &http.Request{URL: &url.URL{}})
	require.NoError(t, err)
	assert.Nil(t, st.gotReportIDs)
}

func TestControllerReportPropagatesStoreError(t *testing.T) {
	c := NewController(NewService(&fakeStore{err: errBoom}))
	_, err := c.GetAverageExecutionTime(context.Background(), &http.Request{URL: &url.URL{}})
	assert.Error(t, err)
}
