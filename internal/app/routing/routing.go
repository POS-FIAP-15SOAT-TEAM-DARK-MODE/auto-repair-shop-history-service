package routing

import (
	"context"
	http2 "net/http"

	api "github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/docs"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/container"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/middleware"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/auth"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/web"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// SetupRouter mounts the middlewares, the operational endpoints and the History API.
func SetupRouter(c *container.HandlersWrapper, m *container.Middlewares) *gin.Engine {
	router := gin.New()

	for name, mid := range *m {
		logger.Global().Info("Starting new middleware", zap.String("name", name))
		router.Use(mid)
	}

	mountSwaggerUI(router)

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http2.StatusOK, gin.H{"message": "pong"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := router.Group("/v1")
	v1.GET("/service-order/:id/history", middleware.Auth(auth.AttendantAndMechanicRoles...), GinHandler(c.HistoryHandler.GetHistory))
	v1.GET("/reports/average-execution-time", middleware.Auth(auth.AttendantRoles...), GinHandler(c.HistoryHandler.GetAverageExecutionTime))

	return router
}

func mountSwaggerUI(router *gin.Engine) {
	router.GET("/swagger.yaml", func(c *gin.Context) {
		c.Data(http2.StatusOK, "application/yaml; charset=utf-8", api.SwaggerBytes)
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger.yaml")))
}

// GinHandler adapts a controller to gin. Route params are copied into the request context under their names.
func GinHandler[T any](cb func(context.Context, *http2.Request) (T, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		for _, v := range c.Params {
			ctx = context.WithValue(ctx, v.Key, v.Value) //nolint:staticcheck // param names are route-scoped string keys
		}

		resp, err := cb(ctx, c.Request)
		if err != nil {
			status, errResp := web.Error(err)
			if web.ShouldLog(err) {
				// ctx carries the request logger, so the line includes request_id.
				logger.Of(ctx).Error(err, zap.Int("status", status))
			}
			c.JSON(status, errResp)
			return
		}
		c.JSON(http2.StatusOK, resp)
	}
}
