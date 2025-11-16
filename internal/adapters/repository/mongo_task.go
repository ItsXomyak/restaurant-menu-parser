package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
)

type MongoTaskRepository struct {
    collection *mongo.Collection
}

func NewMongoTaskRepository(db *mongo.Database) (*MongoTaskRepository, error) {
	collection := db.Collection("parsing_tasks")

	indexModels := []mongo.IndexModel{
		{
			Keys: bson.M{"status": 1},
		},
		{
			Keys: bson.M{"created_at": -1},
		},
	}

	_, err := collection.Indexes().CreateMany(context.Background(), indexModels)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать индексы для parsing_tasks: %w", err)
	}

	return &MongoTaskRepository{
		collection: collection,
	}, nil
}

func (r *MongoTaskRepository) Create(ctx context.Context, task *models.ParsingTask) error {
    _, err := r.collection.InsertOne(ctx, task)
    if err != nil {
        if mongo.IsDuplicateKeyError(err) {
            return fmt.Errorf("%w: %w", types.ErrConflict, err)
        }
        return fmt.Errorf("%w: %w", types.ErrTaskCreateFail, err)
    }
    return nil
}

func (r *MongoTaskRepository) GetByID(ctx context.Context, taskID string) (*models.ParsingTask, error) {
    var task models.ParsingTask

    if err := r.collection.FindOne(ctx, bson.M{"_id": taskID}).Decode(&task); err != nil {
        if errors.Is(err, mongo.ErrNoDocuments) {
            return nil, fmt.Errorf("%w: %w", types.ErrNotFound, err)
        }
        return nil, fmt.Errorf("%w: %w", types.ErrTaskGetFail, err)
    }
    return &task, nil
}

func (r *MongoTaskRepository) Update(ctx context.Context, task *models.ParsingTask) error {
    task.UpdatedAt = time.Now()

    res, err := r.collection.ReplaceOne(ctx, bson.M{"_id": task.ID}, task)
    if err != nil {
        return fmt.Errorf("%w: %w", types.ErrTaskUpdateFail, err)
    }

    if res.MatchedCount == 0 {
        return fmt.Errorf("%w: (MatchedCount=0)", types.ErrNotFound)
    }
    return nil
}