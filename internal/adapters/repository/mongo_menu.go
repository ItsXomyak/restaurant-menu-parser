package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
)

type MongoMenuRepository struct {
    collection *mongo.Collection
}

func NewMongoMenuRepository(db *mongo.Database) (*MongoMenuRepository, error) {
    collection := db.Collection("menus")
    _, err := collection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
        {
            Keys: bson.M{"restaurant_id": 1},
        },
        {
            Keys:    bson.M{"products.ext_id": 1},
            Options: options.Index().SetSparse(true),
        },
    })
    if err != nil {
        return nil, fmt.Errorf("не удалось создать индексы для menus: %w", err)
    }

    return &MongoMenuRepository{
        collection: collection,
    }, nil
}

func (r *MongoMenuRepository) Save(ctx context.Context, menu *models.Menu) error {
    opts := options.Replace().SetUpsert(true)

    _, err := r.collection.ReplaceOne(ctx, bson.M{"_id": menu.ID}, menu, opts)
    if err != nil {
        return fmt.Errorf("%w: %w", types.ErrMenuSaveFail, err)
    }
    return nil
}

func (r *MongoMenuRepository) GetByID(ctx context.Context, menuID primitive.ObjectID) (*models.Menu, error) {
    var menu models.Menu

    if err := r.collection.FindOne(ctx, bson.M{"_id": menuID}).Decode(&menu); err != nil {
        if errors.Is(err, mongo.ErrNoDocuments) {
            return nil, fmt.Errorf("%w: %w", types.ErrNotFound, err)
        }
        return nil, fmt.Errorf("%w: %w", types.ErrMenuGetFail, err)
    }
    return &menu, nil
}

func (r *MongoMenuRepository) UpdateProductStatus(ctx context.Context, productExtID string, status models.ProductStatus) (string, error) {
    filter := bson.M{"products.ext_id": productExtID}
    update := bson.M{
        "$set": bson.M{
            "products.$.status": status,
        },
    }
    opts := options.FindOneAndUpdate().SetReturnDocument(options.Before)

    var originalMenu models.Menu
    err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&originalMenu)

    if err != nil {
        if errors.Is(err, mongo.ErrNoDocuments) {
            return "", fmt.Errorf("%w: продукт %s не найден", types.ErrNotFound, productExtID)
        }
        return "", fmt.Errorf("%w: %w", types.ErrMenuUpdateFail, err)
    }
    for _, product := range originalMenu.Products {
        if product.ExtID == productExtID {
            return string(product.Status), nil
        }
    }
    return "", fmt.Errorf("%w: не удалось найти старый статус для %s", types.ErrInternal, productExtID)
}