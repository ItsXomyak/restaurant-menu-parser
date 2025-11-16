package http

import (
    "context"
    "errors"
    "log/slog"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/ItsXomyak/restaurant-menu-parser/internal/usecase"
    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
)

type TaskHandler struct {
    parsingUseCase *usecase.ParsingUseCase
    logger         *slog.Logger
}

func NewTaskHandler(uc *usecase.ParsingUseCase, logger *slog.Logger) *TaskHandler {
    return &TaskHandler{
        parsingUseCase: uc,
        logger:         logger,
    }
}

func (h *TaskHandler) StartParsingTask(c *gin.Context) {
    var req models.ParseRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        h.logger.Error("Ошибка валидации", slog.Any("error", err))
        c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос: " + err.Error()})
        return
    }
    ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
    defer cancel()
    task, err := h.parsingUseCase.StartParsingTask(ctx, req.SpreadsheetID, req.RestaurantName)
    if err != nil {
        h.logger.Error("Ошибка при запуске задачи", slog.Any("error", err))
        if errors.Is(err, types.ErrConflict) {
            c.JSON(http.StatusConflict, gin.H{"error": "Задача уже существует"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Внутренняя ошибка сервера"})
        }
        return
    }

    c.JSON(http.StatusOK, models.ParseResponse{
        TaskID: task.ID,
        Status: task.Status,
    })
}
func (h *TaskHandler) GetTaskStatus(c *gin.Context) {
    taskID := c.Param("task_id")
    ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    defer cancel()
    task, err := h.parsingUseCase.GetTaskStatus(ctx, taskID)
    if err != nil {
        h.logger.Error("Ошибка получения статуса задачи", slog.Any("error", err))
        if errors.Is(err, types.ErrNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "Задача не найдена"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Внутренняя ошибка сервера"})
        }
        return
    }

    c.JSON(http.StatusOK, task)
}