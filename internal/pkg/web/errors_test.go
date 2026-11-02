package web

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
	"github.com/stretchr/testify/assert"
)

// sensitive mimics a driver message that must never reach a response body.
const sensitive = `pq: duplicate key value violates unique constraint "x" (value)`

func TestErrorMapsServiceOrderIDRequiredTo422(t *testing.T) {
	status, body := Error(history.ErrServiceOrderIDRequired)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, http.StatusUnprocessableEntity, body.StatusCode)
	assert.Equal(t, "service order id is required", body.Error)
	assert.Empty(t, body.Errors)
}

func TestErrorTable(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "domain error keeps its own message",
			err:        history.ErrServiceOrderIDRequired,
			wantStatus: http.StatusUnprocessableEntity,
			wantMsg:    "service order id is required",
		},
		{
			name:       "domain error wrapped with infra text keeps only the domain message",
			err:        fmt.Errorf("%w: %s", history.ErrServiceOrderIDRequired, sensitive),
			wantStatus: http.StatusUnprocessableEntity,
			wantMsg:    "service order id is required",
		},
		{
			name:       "infra 422 is generic",
			err:        fmt.Errorf("write: %w", fmt.Errorf("%w: %s", app.ErrDataViolation, sensitive)),
			wantStatus: http.StatusUnprocessableEntity,
			wantMsg:    "invalid data",
		},
		{
			name:       "unknown error is a generic 500",
			err:        fmt.Errorf("outer: %w", errors.New(sensitive)),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := Error(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantStatus, body.StatusCode)
			assert.Equal(t, tt.wantMsg, body.Error)
			assert.NotContains(t, body.Error, "pq:")
			assert.NotContains(t, body.Error, "unique constraint")
		})
	}
}

func TestErrorDataViolationIsGeneric422(t *testing.T) {
	status, body := Error(fmt.Errorf("wrapped: %w", app.ErrDataViolation))
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "invalid data", body.Error)
}

func TestErrorDefaultsTo500WithoutRootCause(t *testing.T) {
	status, body := Error(fmt.Errorf("outer: %w", errors.New(sensitive)))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "internal server error", body.Error)
	assert.NotContains(t, body.Error, "pq:")
	assert.Empty(t, body.Errors)
}

func TestPublicMessageForConflictStatuses(t *testing.T) {
	// No error maps to 409 today; the generic text is pinned so a future mapping cannot leak the cause.
	assert.Equal(t, "conflict", publicMessage(app.ErrDataConflict, http.StatusConflict))
	assert.Equal(t, "conflict", publicMessage(fmt.Errorf("x: %w", app.ErrInfraConflict), http.StatusConflict))
	assert.Equal(t, "internal server error", publicMessage(errors.New(sensitive), http.StatusBadGateway))
}

func TestShouldLog(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unknown error (500)", errors.New(sensitive), true},
		{"infra data violation (422)", fmt.Errorf("x: %w", app.ErrDataViolation), true},
		{"infra data conflict", fmt.Errorf("x: %w", app.ErrDataConflict), true},
		{"infra conflict", app.ErrInfraConflict, true},
		{"domain error (422)", history.ErrServiceOrderIDRequired, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldLog(tt.err))
		})
	}
}
