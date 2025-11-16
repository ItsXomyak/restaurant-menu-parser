package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoHealthChecker struct {
	client *mongo.Client
}

func NewMongoHealthChecker(client *mongo.Client) *MongoHealthChecker {
	return &MongoHealthChecker{client: client}
}

func (m *MongoHealthChecker) HealthCheck(ctx context.Context) error {
    pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    if err := m.client.Ping(pingCtx, readpref.Primary()); err != nil {
        return fmt.Errorf("mongo ping failed: %w", err)
    }
    return nil
}