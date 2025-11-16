package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Menu struct {
    ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    Name            string             `bson:"name" json:"name"`
    RestaurantID    string             `bson:"restaurant_id" json:"restaurant_id"`
    Products        []Product          `bson:"products" json:"products"`
    AttributeGroups []AttributeGroup   `bson:"attributes_groups" json:"attributes_groups"`
    Attributes      []Attribute        `bson:"attributes" json:"attributes"`
    CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}

type SheetRow struct {
    ProductID string
    ProductName string
    IsCombo bool
    Price float64
    Description string
    AttributeGroupID string
    AttributeGroupName string
    MinSelect int
    MaxSelect int
    AttributeID string
    AttributeName string
    AttributeMin int
    AttributeMax int
    AttributePrice float64
}

type Attribute struct {
    ID      string  `bson:"id" json:"id"`
    GroupID string  `bson:"group_id" json:"group_id"`
    Name    string  `bson:"name" json:"name"`
    Price   float64 `bson:"price" json:"price"`
    Min     int     `bson:"min" json:"min"`
    Max     int     `bson:"max" json:"max"`
}


type Product struct {
    ExtID             string    `bson:"ext_id" json:"ext_id"`
    Name              string    `bson:"name" json:"name"`
    Description       string    `bson:"description,omitempty" json:"description,omitempty"`
    Image             string    `bson:"image,omitempty" json:"image,omitempty"`
    Price             float64   `bson:"price" json:"price"`
    IsCombo           bool      `bson:"is_combo" json:"is_combo"`
    Status            string    `bson:"status" json:"status"`
    AttributeGroupIDs []string  `bson:"attribute_group_ids" json:"attribute_group_ids"`
    CreatedAt         time.Time `bson:"created_at" json:"created_at"`
    UpdatedAt         time.Time `bson:"updated_at" json:"updated_at"`
}

type AttributeGroup struct {
    ID         string `bson:"id" json:"id"`
    Name       string `bson:"name" json:"name"`
    IsRequired bool   `bson:"is_required" json:"is_required"`
    IsMultiple bool   `bson:"is_multiple" json:"is_multiple"`
    MinSelect  int    `bson:"min_select" json:"min_select"`
    MaxSelect  int    `bson:"max_select" json:"max_select"`
}


type ParsingTask struct {
    ID             string             `bson:"_id" json:"id"`
    Status         TaskStatus         `bson:"status" json:"status"`
    SpreadsheetID  string             `bson:"spreadsheet_id" json:"spreadsheet_id"`
    RestaurantName string             `bson:"restaurant_name" json:"restaurant_name"`
    MenuID         primitive.ObjectID `bson:"menu_id,omitempty" json:"menu_id,omitempty"`
    ErrorMessage   string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
    RetryCount     int                `bson:"retry_count" json:"retry_count"`
    CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}

type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

type ProductStatus string

const (
	ProductStatusAvailable    ProductStatus = "available"
	ProductStatusNotAvailable ProductStatus = "not_available"
	ProductStatusDeleted      ProductStatus = "deleted"
)

type ProductStatusAudit struct {
    ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    ProductID string             `bson:"product_id" json:"product_id"`
    EventType string             `bson:"event_type" json:"event_type"`
    OldStatus string             `bson:"old_status,omitempty" json:"old_status,omitempty"`
    NewStatus string             `bson:"new_status" json:"new_status"`
    Reason    string             `bson:"reason,omitempty" json:"reason,omitempty"`
    UserID    string             `bson:"user_id,omitempty" json:"user_id,omitempty"`
    Timestamp time.Time          `bson:"timestamp" json:"timestamp"`
}

type EventType string

const (
	EventProductCreated       EventType = "product.created"
	EventProductUpdated       EventType = "product.updated"
	EventProductStatusChanged EventType = "product.status_changed"
	EventProductDeleted       EventType = "product.deleted"
)

type ProductStatusEvent struct {
    EventType EventType `json:"event_type"`
    ProductID string    `json:"product_id"`
    OldStatus string    `json:"old_status,omitempty"`
    NewStatus string    `json:"new_status"`
    Reason    string    `json:"reason,omitempty"`
    Timestamp time.Time `json:"timestamp"`
    UserID    string    `json:"user_id,omitempty"`
}

type MenuParsingMessage struct {
    TaskID         string    `json:"task_id"`
    SpreadsheetID  string    `json:"spreadsheet_id"`
    RestaurantName string    `json:"restaurant_name"`
    Timestamp      time.Time `json:"timestamp"`
    RetryCount     int       `json:"retry_count"`
}

type ParsedMenuData struct {
    Products        map[string]*Product
    AttributeGroups map[string]*AttributeGroup
    Attributes      map[string]*Attribute
}

type HealthStatus struct {
    Status    string            `json:"status"`
    Timestamp time.Time         `json:"timestamp"`
    Services  map[string]string `json:"services"`
}

type ParseRequest struct {
    SpreadsheetID  string `json:"spreadsheet_id" binding:"required"`
    RestaurantName string `json:"restaurant_name" binding:"required"`
}

type ParseResponse struct {
    TaskID string     `json:"task_id"`
    Status TaskStatus `json:"status"`
}

type TaskStatusResponse struct {
    TaskID    string             `json:"task_id"`
    Status    TaskStatus         `json:"status"`
    MenuID    primitive.ObjectID `json:"menu_id,omitempty"`
    Error     string             `json:"error,omitempty"`
    CreatedAt time.Time          `json:"created_at"`
    UpdatedAt time.Time          `json:"updated_at"`
}

type UpdateProductStatusRequest struct {
    Status ProductStatus `json:"status" binding:"required,oneof=available not_available deleted"`
    Reason string        `json:"reason,omitempty"`
    UserID string        `json:"user_id,omitempty"`
}

type UpdateProductStatusResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}