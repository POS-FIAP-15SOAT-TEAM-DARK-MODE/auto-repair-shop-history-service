package web

import (
	"errors"
	"net/http"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
)

type errorResponse struct {
	StatusCode int      `json:"code"`
	Errors     []string `json:"errors,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// Error builds the error body and the HTTP status for err.
func Error(err error) (int, errorResponse) {
	status := getHTTPStatus(err)
	stacked, msg := buildErrorMessages(err, status)
	return status, errorResponse{StatusCode: status, Errors: stacked, Error: msg}
}

func getHTTPStatus(err error) int {
	switch {
	case errors.Is(err, history.ErrServiceOrderIDRequired):
		return http.StatusUnprocessableEntity
	case errors.Is(err, app.ErrDataViolation):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// buildErrorMessages returns the innermost message for 422/500, as the monolith does.
func buildErrorMessages(err error, status int) ([]string, string) {
	if status == http.StatusInternalServerError || status == http.StatusUnprocessableEntity {
		root := err
		for {
			unwrapped := errors.Unwrap(root)
			if unwrapped == nil {
				break
			}
			root = unwrapped
		}
		return nil, root.Error()
	}
	return nil, err.Error()
}
