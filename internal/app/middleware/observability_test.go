package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/middleware"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggerMiddleware_EventsCarryRequestIDWithoutSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.DebugLevel)
	restore := logger.ReplaceGlobal(zap.New(core).With(zap.String("service", logger.ServiceName)))
	t.Cleanup(restore)

	router := gin.New()
	router.Use(middleware.Logger())
	router.GET("/v1/reports/average-execution-time", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/average-execution-time?work_id=w1", nil)
	req.Header.Set("X-Request-Id", "req-42")
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.secret-payload.signature")
	router.ServeHTTP(httptest.NewRecorder(), req)

	all := logs.All()
	require.Len(t, all, 2)
	assert.Equal(t, "request.started", all[0].Message)
	assert.Equal(t, "request.completed", all[1].Message)
	for _, e := range all {
		assert.Equal(t, "req-42", e.ContextMap()["request_id"])
		assert.Equal(t, logger.ServiceName, e.ContextMap()["service"])
		for k, v := range e.ContextMap() {
			assert.NotContains(t, fmt.Sprint(v), "secret-payload", "field %s leaked the token", k)
		}
	}
}
