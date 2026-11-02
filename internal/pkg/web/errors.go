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

// domainErrors are returned on purpose by the domain: their own message is safe to show to the client.
var domainErrors = []error{
	history.ErrServiceOrderIDRequired,
}

// infraErrors are generic failures of the infrastructure. Their causes never reach the response body.
var infraErrors = []error{
	app.ErrDataConflict,
	app.ErrInfraConflict,
	app.ErrDataViolation,
}

// Error builds the error body and the HTTP status for err. The body never carries an infrastructure cause (for
// example, a Postgres or DynamoDB driver message): only domain errors keep their own message, the rest gets a
// generic one. The full error goes to the server log (see ShouldLog).
func Error(err error) (int, errorResponse) {
	status := getHTTPStatus(err)
	return status, errorResponse{StatusCode: status, Error: publicMessage(err, status)}
}

// ShouldLog reports whether err must be written to the server log at error level: every 5xx and the infrastructure
// 409/422 failures. Domain errors are only debug-logged by the controllers.
func ShouldLog(err error) bool {
	if getHTTPStatus(err) >= http.StatusInternalServerError {
		return true
	}
	for _, infra := range infraErrors {
		if errors.Is(err, infra) {
			return true
		}
	}
	return false
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

// publicMessage returns the message the client may read for err at status.
func publicMessage(err error, status int) string {
	for _, domain := range domainErrors {
		if errors.Is(err, domain) {
			return domain.Error()
		}
	}
	switch status {
	case http.StatusUnprocessableEntity:
		return "invalid data"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal server error"
	}
}
