package history

import (
	"context"
	"net/http"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
	"go.uber.org/zap"
)

// Controller exposes the History queries as HTTP handlers.
type Controller struct {
	svc *Service
}

// NewController returns a Controller over svc.
func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

// GetHistory handles GET /v1/service-order/:id/history. The route param id is read from the context.
func (c *Controller) GetHistory(ctx context.Context, _ *http.Request) ([]HistoryDTO, error) {
	id, _ := ctx.Value("id").(string) //nolint:staticcheck // gin route params are passed as string keys by GinHandler
	entries, err := c.svc.GetHistory(ctx, id)
	if err != nil {
		logger.Of(ctx).Debug("history.query_failed",
			zap.String("operation", "get_service_order_history"), zap.Error(err))
		return nil, err
	}
	return MapHistory(entries), nil
}

// GetAverageExecutionTime handles GET /v1/reports/average-execution-time?work_id=...
func (c *Controller) GetAverageExecutionTime(ctx context.Context, req *http.Request) ([]ReportDTO, error) {
	workIDs := req.URL.Query()["work_id"]
	entries, err := c.svc.GetAverageExecutionTime(ctx, workIDs)
	if err != nil {
		logger.Of(ctx).Debug("report.query_failed",
			zap.String("operation", "get_average_execution_time"), zap.Error(err))
		return nil, err
	}
	return MapReport(entries), nil
}
