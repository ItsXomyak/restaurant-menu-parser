package http

import (
    "context"
    "errors"
    "log/slog"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
    "github.com/ItsXomyak/restaurant-menu-parser/internal/usecase"
)

type ProductHandler struct {
    productUseCase *usecase.ProductUseCase
    logger         *slog.Logger
}

func NewProductHandler(uc *usecase.ProductUseCase, logger *slog.Logger) *ProductHandler {
    return &ProductHandler{
        productUseCase: uc,
        logger:         logger,
    }
}

func (h *ProductHandler) GetMenuByID(c *gin.Context) {
    menuID := c.Param("menu_id")
    ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    defer cancel()
    menu, err := h.productUseCase.GetMenuByID(ctx, menuID)
    if err != nil {
        h.logger.Error("Ошибка получения меню", slog.Any("error", err))
        switch {
        case errors.Is(err, types.ErrNotFound):
            c.JSON(http.StatusNotFound, gin.H{"error": "Меню не найдено"})
        case errors.Is(err, types.ErrBadRequest):
            c.JSON(http.StatusBadRequest, gin.H{"error": "Невалидный ID меню"})
        default:
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Внутренняя ошибка сервера"})
        }
        return
    }
    c.JSON(http.StatusOK, menu)
}
func (h *ProductHandler) QueueStatusUpdate(c *gin.Context) {
    productID := c.Param("product_id")
    var req models.UpdateProductStatusRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        h.logger.Error("Ошибка валидации", slog.Any("error", err))
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Некорректный запрос: " + err.Error(),
        })
        return
    }
    ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    defer cancel()
    if err := h.productUseCase.QueueStatusUpdate(ctx, productID, req); err != nil {
        h.logger.Error("Ошибка постановки задачи в очередь", slog.Any("error", err))
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Внутренняя ошибка сервера",
        })
        return
    }
    c.JSON(http.StatusAccepted, models.UpdateProductStatusResponse{
        Success: true,
        Message: "Status update queued",
    })
}
