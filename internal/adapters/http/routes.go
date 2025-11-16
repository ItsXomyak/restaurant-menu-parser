package http

import "github.com/gin-gonic/gin"

func (h *TaskHandler) RegisterRoutes(router *gin.Engine) {
    v1 := router.Group("/api/v1")
    {
        v1.POST("/parse", h.StartParsingTask)
        v1.GET("/parse/:task_id", h.GetTaskStatus)
    }
}

func (h *ProductHandler) RegisterRoutes(router *gin.Engine) {
    v1 := router.Group("/api/v1")
    {
        v1.GET("/menu/:menu_id", h.GetMenuByID)
        v1.PATCH("/products/:product_id/status", h.QueueStatusUpdate)
    }
}