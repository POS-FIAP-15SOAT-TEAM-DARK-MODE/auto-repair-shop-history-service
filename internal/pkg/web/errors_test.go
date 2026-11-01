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

func TestErrorMapsServiceOrderIDRequiredTo422(t *testing.T) {
	status, body := Error(history.ErrServiceOrderIDRequired)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, http.StatusUnprocessableEntity, body.StatusCode)
	assert.Equal(t, "service order id is required", body.Error)
}

func TestErrorMapsDataViolationTo422(t *testing.T) {
	status, _ := Error(fmt.Errorf("wrapped: %w", app.ErrDataViolation))
	assert.Equal(t, http.StatusUnprocessableEntity, status)
}

func TestErrorDefaultsTo500WithRootCause(t *testing.T) {
	root := errors.New("root cause")
	status, body := Error(fmt.Errorf("outer: %w", root))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "root cause", body.Error)
	assert.Empty(t, body.Errors)
}
