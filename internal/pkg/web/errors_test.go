package web

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/consumer"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
	jsonv2 "github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/json"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

// sensitive mimics a driver message that must never reach a response body.
const sensitive = `pq: duplicate key value violates unique constraint "x" (value)`

// errRootCause is an infrastructure error used as the unknown leaf of joins.
var errRootCause = errors.New(sensitive)

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

func TestBuildMessagesForConflictStatus(t *testing.T) {
	// No error maps to 409 today; the generic text is pinned so a future mapping cannot leak the cause.
	_, msg := buildErrorMessages(app.ErrDataConflict, http.StatusConflict)
	assert.Equal(t, "conflict", msg)
	_, msg = buildErrorMessages(fmt.Errorf("x: %w", app.ErrInfraConflict), http.StatusConflict)
	assert.Equal(t, "conflict", msg)
	_, msg = buildErrorMessages(errRootCause, http.StatusBadGateway)
	assert.Equal(t, "internal server error", msg)
}

func TestValidationSentinelAnswers400WithItsText(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"consumer invalid payload", consumer.ErrInvalidPayload, "invalid message payload"},
		{"consumer invalid payload wrapped", fmt.Errorf("%w: new is required", consumer.ErrInvalidPayload), "invalid message payload"},
		{"envelope", messaging.ErrInvalidEnvelope, "invalid message envelope"},
		{"json syntax", jsonv2.ErrJSONSyntax, "invalid JSON syntax"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := Error(tc.err)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, http.StatusBadRequest, body.StatusCode)
			assert.Empty(t, body.Error)
			assert.Equal(t, []string{tc.want}, body.Errors)
		})
	}
}

func TestJoinOfValidationSentinelsAnswers400WithOnlySentinelTexts(t *testing.T) {
	status, body := Error(errors.Join(consumer.ErrInvalidPayload, messaging.ErrInvalidEnvelope))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Empty(t, body.Error)
	assert.Equal(t, []string{"invalid message payload", "invalid message envelope"}, body.Errors)
}

func TestValidationListKeepsOnlySentinelTexts(t *testing.T) {
	// The driver text is only in the message of a %s argument, not in the wrap chain: the sentinel alone is the leaf.
	status, body := Error(fmt.Errorf("%w: %s", consumer.ErrInvalidPayload, sensitive))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, []string{"invalid message payload"}, body.Errors)
	assert.NotContains(t, fmt.Sprint(body.Errors), "pq:")
}

func TestOnlyInfraOrUnknownCausesAnswerGeneric500(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"single unknown cause", errRootCause},
		{"unknown wrap", fmt.Errorf("save: %w", errRootCause)},
		{"two unknown causes", errors.Join(errors.New("a"), errRootCause)},
		{"infra wrapped with infra", fmt.Errorf("%w: %w", errors.New("a"), errRootCause)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := Error(tc.err)
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.Equal(t, http.StatusInternalServerError, body.StatusCode)
			assert.Equal(t, "internal server error", body.Error)
			assert.Empty(t, body.Errors)
			assert.NotContains(t, body.Error, "pq:")
			assert.NotContains(t, body.Error, "unique constraint")
		})
	}
}

func TestValidationWithInfraAnswers400WithOnlySentinelText(t *testing.T) {
	// A validation sentinel anywhere in the error gives 400; the driver text must never appear in errors[].
	cases := []struct {
		name string
		err  error
	}{
		{"joined with infra", errors.Join(consumer.ErrInvalidPayload, errRootCause)},
		{"%w: %w with infra", fmt.Errorf("%w: %w", consumer.ErrInvalidPayload, errRootCause)},
		{"json syntax joined with infra", errors.Join(jsonv2.ErrJSONSyntax, errRootCause)},
		{"infra wrapped with validation", fmt.Errorf("save: %w", fmt.Errorf("%w: %w", errRootCause, consumer.ErrInvalidPayload))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := Error(tc.err)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, http.StatusBadRequest, body.StatusCode)
			assert.Empty(t, body.Error)
			assert.Len(t, body.Errors, 1)
			assert.NotContains(t, fmt.Sprint(body.Errors), "pq:")
			assert.NotContains(t, fmt.Sprint(body.Errors), "unique constraint")
		})
	}
}

func TestValidationWithDomainJoinFollowsValidationRule(t *testing.T) {
	status, body := Error(errors.Join(history.ErrServiceOrderIDRequired, consumer.ErrInvalidPayload))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Empty(t, body.Error)
	assert.Equal(t, []string{"invalid message payload"}, body.Errors)
}

func TestDomainJoinedWithInfraKeepsDomainMessage(t *testing.T) {
	status, body := Error(errors.Join(history.ErrServiceOrderIDRequired, errRootCause))
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "service order id is required", body.Error)
	assert.Empty(t, body.Errors)
	assert.NotContains(t, body.Error, "pq:")
}

func TestShouldLog(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unknown error (500)", errRootCause, true},
		{"infra data violation (422)", fmt.Errorf("x: %w", app.ErrDataViolation), true},
		{"infra data conflict", fmt.Errorf("x: %w", app.ErrDataConflict), true},
		{"infra conflict", app.ErrInfraConflict, true},
		{"validation joined with infra (400)", errors.Join(consumer.ErrInvalidPayload, errRootCause), false},
		{"pure validation (400)", consumer.ErrInvalidPayload, false},
		{"domain error (422)", history.ErrServiceOrderIDRequired, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldLog(tt.err))
		})
	}
}

func TestLeavesFlattensJoinsAndWraps(t *testing.T) {
	joined := errors.Join(consumer.ErrInvalidPayload, fmt.Errorf("ctx: %w", errRootCause))
	assert.Len(t, leaves(joined), 2)
	assert.Nil(t, leaves(nil))
	assert.True(t, errors.Is(leaves(fmt.Errorf("ctx: %w", consumer.ErrInvalidPayload))[0], consumer.ErrInvalidPayload))
}
