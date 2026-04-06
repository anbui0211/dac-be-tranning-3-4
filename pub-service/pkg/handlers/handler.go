package handlers

import (
	"net/http"

	services "pub-service/pkg/services"

	"github.com/gin-gonic/gin"
)

type Handler interface {
	ProcessBatchHandler(c *gin.Context)
	UploadAssetsHandler(c *gin.Context)
}

type HandlerImpl struct {
	service services.Service
}

func NewHandler(service services.Service) Handler {
	return &HandlerImpl{
		service: service,
	}
}

func (h *HandlerImpl) ProcessBatchHandler(c *gin.Context) {
	if err := h.service.ProcessBatch(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Batch processing started"})
}

func (h *HandlerImpl) UploadAssetsHandler(c *gin.Context) {
	results, err := h.service.UploadAssets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Assets uploaded successfully",
		"results": results,
	})
}
