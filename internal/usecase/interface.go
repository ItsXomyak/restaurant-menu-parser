package usecase

import (
    "context"

    "go.mongodb.org/mongo-driver/bson/primitive"

    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
)

type AuditRepository interface {
    Create(ctx context.Context, auditEvent *models.ProductStatusAudit) error
}

type TaskRepository interface {
    Create(ctx context.Context, task *models.ParsingTask) error
    GetByID(ctx context.Context, taskID string) (*models.ParsingTask, error)
    Update(ctx context.Context, task *models.ParsingTask) error
}

type MenuRepository interface {
    Save(ctx context.Context, menu *models.Menu) error
    GetByID(ctx context.Context, menuID primitive.ObjectID) (*models.Menu, error)
    UpdateProductStatus(ctx context.Context, productExtID string, status models.ProductStatus) (oldStatus string, err error)
}

type MessagePublisher interface {
    DeclareQueue(ctx context.Context, queueName string, withDLX bool) error
    Publish(ctx context.Context, queueName string, body []byte) error
    Close(ctx context.Context) error
}

type SpreadsheetParser interface {
    ParseMenu(ctx context.Context, spreadsheetID string) (*models.ParsedMenuData, error)
}

type HealthChecker interface {
    HealthCheck(ctx context.Context) error
}