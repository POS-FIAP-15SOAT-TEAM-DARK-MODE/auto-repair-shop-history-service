package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				// The panic value may not be an error, so it is wrapped instead of asserted.
				logger.Of(c).Error(fmt.Errorf("panic: %v", err),
					zap.String("behavior", "[PANIC RECOVERED]"),
					zap.String("stack", string(stack)),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":  http.StatusInternalServerError,
					"error": "internal server error",
				})
			}
		}()

		c.Next()
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		status := c.Writer.Status()
		if status == http.StatusOK {
			status = http.StatusInternalServerError
		}

		// The handler's errors are written to the server log by the logger middleware, never to the client.
		c.JSON(status, gin.H{
			"code":  status,
			"error": http.StatusText(status),
		})
	}
}
