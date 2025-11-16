package usecase

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"

    "go.mongodb.org/mongo-driver/bson/primitive"

    "github.com/google/uuid"

    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
    "github.com/ItsXomyak/restaurant-menu-parser/pkg/trm"
    "github.com/ItsXomyak/restaurant-menu-parser/pkg/logger"
)



type ParsingUseCase struct {
    taskRepo  TaskRepository
    menuRepo  MenuRepository
    parser    SpreadsheetParser
    publisher MessagePublisher
    trm       trm.TxManager
}

func NewParsingUseCase(
    tr TaskRepository,
    p MessagePublisher,
    trm trm.TxManager,
    parser SpreadsheetParser,
    menuRepo MenuRepository,
) *ParsingUseCase {
    return &ParsingUseCase{
        taskRepo:  tr,
        menuRepo:  menuRepo,
        parser:    parser,
        publisher: p,
        trm:       trm,
    }
}

func (uc *ParsingUseCase) StartParsingTask(ctx context.Context, spreadsheetID string, restaurantName string) (*models.ParsingTask, error) {
    log := logger.LoggerFromContext(ctx)
    var task *models.ParsingTask
    now := time.Now()

    err := uc.trm.Do(ctx, func(txCtx context.Context) error {
        log.Info("Создание задачи парсинга", "spreadsheet_id", spreadsheetID, "restaurant", restaurantName)
        task = &models.ParsingTask{
            ID:             uuid.NewString(),
            Status:         models.TaskStatusQueued,
            SpreadsheetID:  spreadsheetID,
            RestaurantName: restaurantName,
            RetryCount:     0,
            CreatedAt:      now,
            UpdatedAt:      now,
        }

        if err := uc.taskRepo.Create(txCtx, task); err != nil {
            log.Error("Ошибка создания задачи", slog.Any("error", err))
            return err
        }

        msg := models.MenuParsingMessage{
            TaskID:         task.ID,
            SpreadsheetID:  task.SpreadsheetID,
            RestaurantName: task.RestaurantName,
            Timestamp:      now,
            RetryCount:     0,
        }

        msgBytes, err := json.Marshal(msg)
        if err != nil {
            log.Error("Ошибка сериализации сообщения", slog.Any("error", err))
            return err
        }

        queueName := "menu-parsing"
        if err := uc.publisher.Publish(txCtx, queueName, msgBytes); err != nil {
            log.Error("Ошибка публикации сообщения", "queue", queueName, slog.Any("error", err))
            return err
        }
        return nil
    })

    if err != nil {
        log.Error("Ошибка постановки задачи", slog.Any("error", err))
        return nil, err
    }

    return task, nil
}

func (uc *ParsingUseCase) ProcessParsingTask(ctx context.Context, msg models.MenuParsingMessage) error {
    log := logger.LoggerFromContext(ctx)
    task, err := uc.taskRepo.GetByID(ctx, msg.TaskID)
    if err != nil {
        log.Error("Ошибка получения задачи", "task_id", msg.TaskID, slog.Any("error", err))
        return err
    }

    err = uc.trm.Do(ctx, func(txCtx context.Context) error {
        log.Info("Начало обработки задачи", "task_id", msg.TaskID)
        task.Status = models.TaskStatusProcessing
        task.UpdatedAt = time.Now()
        if err := uc.taskRepo.Update(txCtx, task); err != nil {
            log.Error("Ошибка обновления статуса задачи", "task_id", msg.TaskID, slog.Any("error", err))
            return err
        }

        menuData, err := uc.parser.ParseMenu(txCtx, msg.SpreadsheetID)
        if err != nil {
            log.Error("Ошибка парсинга меню", "spreadsheet_id", msg.SpreadsheetID, slog.Any("error", err))
            return err
        }

        now := time.Now()
        menu := &models.Menu{
            ID:              primitive.NewObjectID(),
            Name:            task.RestaurantName,
            Products:        flattenProducts(menuData.Products),
            AttributeGroups: flattenAttributeGroups(menuData.AttributeGroups),
            Attributes:      flattenAttributes(menuData.Attributes),
            CreatedAt:       now,
            UpdatedAt:       now,
        }
        if err := uc.menuRepo.Save(txCtx, menu); err != nil {
            log.Error("Ошибка сохранения меню", slog.Any("error", err))
            return err
        }
        task.Status = models.TaskStatusCompleted
        task.MenuID = menu.ID
        task.ErrorMessage = ""
        task.UpdatedAt = time.Now()
        if err := uc.taskRepo.Update(txCtx, task); err != nil {
            log.Error("Ошибка обновления задачи на completed", slog.Any("error", err))
            return err
        }
        return nil
    })

    if err != nil {
        log.Error("Ошибка обработки задачи, переводим в failed", "task_id", msg.TaskID, slog.Any("error", err))
        task.Status = models.TaskStatusFailed
        task.ErrorMessage = err.Error()
        task.UpdatedAt = time.Now()
        if updateErr := uc.taskRepo.Update(ctx, task); updateErr != nil {
            log.Error("Ошибка обновления задачи на failed", slog.Any("original_error", err), slog.Any("update_error", updateErr))
            return err
        }
        return err
    }

    return nil
}

func (uc *ParsingUseCase) GetTaskStatus(ctx context.Context, taskID string) (*models.ParsingTask, error) {
    var task *models.ParsingTask
    var err error
    err = uc.trm.DoReadOnly(ctx, func(txCtx context.Context) error {
        task, err = uc.taskRepo.GetByID(txCtx, taskID)
        if err != nil {
            return err
        }
        return nil
    })

    if err != nil {
        return nil, err
    }

    return task, nil
}

func (uc *ProductUseCase) GetMenuByID(ctx context.Context, menuIDHex string) (*models.Menu, error) {
    var menu *models.Menu
    var err error
    menuID, err := primitive.ObjectIDFromHex(menuIDHex)
    if err != nil {
        return nil, types.ErrBadRequest
    }
    err = uc.trm.DoReadOnly(ctx, func(txCtx context.Context) error {
        menu, err = uc.menuRepo.GetByID(txCtx, menuID)
        if err != nil {
            return err
        }
        return nil
    })

    if err != nil {
        return nil, err
    }

    return menu, nil
}


func flattenProducts(m map[string]*models.Product) []models.Product {
    s := make([]models.Product, 0, len(m))
    for _, v := range m {
        s = append(s, *v)
    }
    return s
}

func flattenAttributeGroups(m map[string]*models.AttributeGroup) []models.AttributeGroup {
    s := make([]models.AttributeGroup, 0, len(m))
    for _, v := range m {
        s = append(s, *v)
    }
    return s
}

func flattenAttributes(m map[string]*models.Attribute) []models.Attribute {
    s := make([]models.Attribute, 0, len(m))
    for _, v := range m {
        s = append(s, *v)
    }
    return s
}




