package web

import (
	"errors"
	"net/http"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/consumer"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
	jsonv2 "github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/json"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
)

type errorResponse struct {
	StatusCode int      `json:"code"`
	Errors     []string `json:"errors,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// Error builds the error body and the HTTP status for err. The body never carries an infrastructure cause (for
// example, a Postgres or DynamoDB driver message): only domain and validation sentinels reach the client. The full
// error goes to the server log (see ShouldLog).
func Error(err error) (int, errorResponse) {
	status := getHTTPStatus(err)
	stacked, msg := buildErrorMessages(err, status)
	return status, errorResponse{StatusCode: status, Errors: stacked, Error: msg}
}

// ShouldLog reports whether err must be written to the server log at error level: every 5xx and the infrastructure
// 409/422 failures. Domain and validation errors are only debug-logged by the controllers.
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

// validationErrors are the input-validation sentinels: they answer 400 and their messages go to errors[].
var validationErrors = []error{
	jsonv2.ErrJSONSyntax,
	jsonv2.ErrJSONType,
	jsonv2.ErrJSONUnexpectedEOF,
	jsonv2.ErrJSONEmptyBody,
	jsonv2.ErrWrongPayloadFormat,
	consumer.ErrInvalidPayload,
	messaging.ErrInvalidEnvelope,
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

func getHTTPStatus(err error) int {
	switch {
	case isValidationError(err):
		return http.StatusBadRequest
	case errors.Is(err, history.ErrServiceOrderIDRequired):
		return http.StatusUnprocessableEntity
	case errors.Is(err, app.ErrDataViolation):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// buildErrorMessages returns the body fields for err at status. A 400 lists only the texts of the validation
// sentinels matched by err; every other status gets a single message.
func buildErrorMessages(err error, status int) ([]string, string) {
	switch status {
	case http.StatusBadRequest:
		return validationMessages(err), ""
	case http.StatusUnprocessableEntity:
		if msg, ok := domainMessage(err); ok {
			return nil, msg
		}
		return nil, "invalid data"
	case http.StatusConflict:
		if msg, ok := domainMessage(err); ok {
			return nil, msg
		}
		return nil, "conflict"
	default:
		return nil, "internal server error"
	}
}

// domainMessage returns the message of the first domain sentinel that err matches.
func domainMessage(err error) (string, bool) {
	for _, domain := range domainErrors {
		if errors.Is(err, domain) {
			return domain.Error(), true
		}
	}
	return "", false
}

// isValidationError reports whether err is a pure validation error: see validationMessages.
func isValidationError(err error) bool {
	return len(validationMessages(err)) > 0
}

// validationMessages returns the texts of the validation sentinels matched by the leaves of err. Any other cause in
// the same error (an infrastructure or unknown leaf, even one joined with a validation sentinel) is never included:
// only the sentinel texts reach errors[]. An error with no validation sentinel returns nil.
func validationMessages(err error) []string {
	var msgs []string
	for _, leaf := range leaves(err) {
		sentinel, ok := matchValidation(leaf)
		if !ok {
			continue
		}
		if !containsText(msgs, sentinel.Error()) {
			msgs = append(msgs, sentinel.Error())
		}
	}
	return msgs
}

// matchValidation returns the first validation sentinel that leaf matches.
func matchValidation(leaf error) (error, bool) {
	for _, v := range validationErrors {
		if errors.Is(leaf, v) {
			return v, true
		}
	}
	return nil, false
}

// leaves returns the causes at the end of err's wrap tree. Joined errors (Unwrap() []error) contribute every branch;
// a single wrap contributes what it wraps.
func leaves(err error) []error {
	if err == nil {
		return nil
	}
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		var out []error
		for _, child := range wrapped.Unwrap() {
			out = append(out, leaves(child)...)
		}
		return out
	case interface{ Unwrap() error }:
		if inner := wrapped.Unwrap(); inner != nil {
			return leaves(inner)
		}
	}
	return []error{err}
}

func containsText(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
