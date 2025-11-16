package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
	"github.com/ItsXomyak/restaurant-menu-parser/pkg/logger"
	"github.com/ItsXomyak/restaurant-menu-parser/pkg/trm"
)

type ProductUseCase struct {
    publisher MessagePublisher
    menuRepo  MenuRepository
    auditRepo AuditRepository
    trm       trm.TxManager
}

func NewProductUseCase(
    p MessagePublisher,
    mr MenuRepository,
    ar AuditRepository,
    trm trm.TxManager,
) *ProductUseCase {
	return &ProductUseCase{
		publisher: p,
		menuRepo:  mr,
		auditRepo: ar,
		trm:       trm,
	}
}

func (uc *ProductUseCase) QueueStatusUpdate(ctx context.Context, productID string, req models.UpdateProductStatusRequest) error {
    log := logger.LoggerFromContext(ctx)

    event := models.ProductStatusEvent{
        EventType: models.EventProductStatusChanged,
        ProductID: productID,
        NewStatus: string(req.Status),
        Reason:    req.Reason,
        UserID:    req.UserID,
        Timestamp: time.Now(),
    }

    msgBytes, err := json.Marshal(event)
    if err != nil {
        log.Error("Ошибка сериализации события", slog.Any("error", err))
        return fmt.Errorf("%w: %w", types.ErrInternal, err)
    }

    queueName := "product-status"
    if err := uc.publisher.Publish(ctx, queueName, msgBytes); err != nil {
        log.Error("Ошибка публикации события", "queue", queueName, slog.Any("error", err))
        return err
    }

    return nil
}

func (uc *ProductUseCase) ProcessStatusUpdate(ctx context.Context, event models.ProductStatusEvent) error {
    log := logger.LoggerFromContext(ctx)
    
    log.Info("Starting transaction", 
        "product_id", event.ProductID,
        "new_status", event.NewStatus)

    err := uc.trm.Do(ctx, func(txCtx context.Context) error {
        if sess := mongo.SessionFromContext(txCtx); sess != nil {
            log.Debug("Session found in context - transaction is active")
        } else {
            log.Warn("No session in context - NOT in transaction!")
        }

        log.Debug("Updating product status...")
        oldStatus, err := uc.menuRepo.UpdateProductStatus(
            txCtx,
            event.ProductID,
            models.ProductStatus(event.NewStatus),
        )
        if err != nil {
            log.Error("Failed to update product status", 
                "product_id", event.ProductID, 
                "error", err)
            return fmt.Errorf("update product status: %w", err)
        }
        log.Info("✅ Product status updated", 
            "product_id", event.ProductID,
            "old_status", oldStatus, 
            "new_status", event.NewStatus)

        auditLog := &models.ProductStatusAudit{
            ID:        primitive.NewObjectID(),
            ProductID: event.ProductID,
            EventType: string(event.EventType),
            OldStatus: oldStatus,
            NewStatus: event.NewStatus,
            Reason:    event.Reason,
            UserID:    event.UserID,
            Timestamp: event.Timestamp,
        }

        log.Debug("📋 Creating audit log...", "audit_id", auditLog.ID.Hex())
        if err := uc.auditRepo.Create(txCtx, auditLog); err != nil {
            log.Error("AUDIT CREATE FAILED - TRANSACTION SHOULD ROLLBACK", 
                "audit_id", auditLog.ID.Hex(),
                "product_id", event.ProductID,
                "error", err)
            return fmt.Errorf("create audit log: %w", err)
        }
        log.Info("Audit log created", "audit_id", auditLog.ID.Hex())

        log.Debug("Transaction function completed successfully")
        return nil
    })

    if err != nil {
        log.Error("Transaction FAILED and ROLLED BACK", 
            "product_id", event.ProductID,
            "error", err)
        return err
    }
    
    log.Info("Transaction COMMITTED successfully", 
        "product_id", event.ProductID)
    return nil
}