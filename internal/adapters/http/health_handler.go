package http

import (
    "context"
    "log/slog"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/ItsXomyak/restaurant-menu-parser/internal/usecase"
)

type HealthHandler struct {
    healthUseCase *usecase.HealthUseCase
    logger        *slog.Logger
}

func NewHealthHandler(uc *usecase.HealthUseCase, logger *slog.Logger) *HealthHandler {
    return &HealthHandler{
        healthUseCase: uc,
        logger:        logger,
    }
}

func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
    v1 := router.Group("/api/v1")
    {
        v1.GET("/health", h.CheckHealth)
    }
}
func (h *HealthHandler) CheckHealth(c *gin.Context) {
    ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    defer cancel()
    status := h.healthUseCase.Check(ctx)
    if status.Status != "healthy" {
        h.logger.Error("Health check failed", slog.Any("services", status.Services))
        c.JSON(http.StatusServiceUnavailable, status)
        return
    }
    c.JSON(http.StatusOK, status)
}
