package usecase

import (
    "context"
    "log/slog"
    "time"

    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
    "github.com/ItsXomyak/restaurant-menu-parser/pkg/logger"
)

type HealthUseCase struct {
    dbHealth    HealthChecker
    queueHealth HealthChecker
}

func NewHealthUseCase(
    dbHealth HealthChecker,
    queueHealth HealthChecker,
) *HealthUseCase {
    return &HealthUseCase{
        dbHealth:    dbHealth,
        queueHealth: queueHealth,
    }
}

func (uc *HealthUseCase) Check(ctx context.Context) *models.HealthStatus {
    log := logger.LoggerFromContext(ctx)
    status := &models.HealthStatus{
        Status:    "healthy",
        Timestamp: time.Now(),
        Services:  make(map[string]string),
    }
    if err := uc.dbHealth.HealthCheck(ctx); err != nil {
        status.Status = "unhealthy"
        status.Services["database"] = "failed: " + err.Error()
        log.Error("Database health failed", slog.Any("error", err))
    } else {
        status.Services["database"] = "ok"
    }
    if err := uc.queueHealth.HealthCheck(ctx); err != nil {
        status.Status = "unhealthy"
        status.Services["queue"] = "failed: " + err.Error()
        log.Error("Queue health failed", slog.Any("error", err))
    } else {
        status.Services["queue"] = "ok"
    }

    return status
}