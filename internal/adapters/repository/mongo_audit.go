package repository

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
)

type MongoAuditRepository struct {
    collection *mongo.Collection
}

func NewMongoAuditRepository(db *mongo.Database) (*MongoAuditRepository, error) {
    collection := db.Collection("product_status_audit")

    _, err := collection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
        {
            Keys: bson.M{"product_id": 1},
        },
        {
            Keys: bson.M{"timestamp": -1},
        },
    })
    if err != nil {
        return nil, fmt.Errorf("не удалось создать индексы для product_status_audit: %w", err)
    }

    return &MongoAuditRepository{
        collection: collection,
    }, nil
}

func (r *MongoAuditRepository) Create(ctx context.Context, auditEvent *models.ProductStatusAudit) error {
    res, err := r.collection.InsertOne(ctx, auditEvent)
    if err != nil {
        return fmt.Errorf("%w: %w", types.ErrAuditCreateFail, err)
    }

    if res.InsertedID == nil {
        return fmt.Errorf("%w: MongoDB вернул nil ID при вставке", types.ErrAuditCreateFail)
    }
    
    return nil
}