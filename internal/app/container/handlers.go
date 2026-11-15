package container

import (
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
	"github.com/gin-gonic/gin"
)

// HandlersWrapper groups the HTTP controllers mounted by the router.
type HandlersWrapper struct {
	HistoryHandler *history.Controller
}

// Middlewares maps a name to a gin middleware; the router mounts them in the map's iteration order.
type Middlewares map[string]gin.HandlerFunc
