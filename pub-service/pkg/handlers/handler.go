package handlers

import (
	"log"
	"net/http"

	services "pub-service/pkg/services"

	"github.com/gin-gonic/gin"
)

type Handler interface {
	HealthHandler(c *gin.Context)
	UploadFileHandler(c *gin.Context)
	ProcessBatchHandler(c *gin.Context)
	ListFilesHandler(c *gin.Context)
	UploadMultipleHandler(c *gin.Context)
	UploadAllHandler(c *gin.Context)
}

type HandlerImpl struct {
	service services.Service
}

func NewHandler(service services.Service) Handler {
	return &HandlerImpl{
		service: service,
	}
}

func (h *HandlerImpl) HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pub-service"})
}

func (h *HandlerImpl) UploadFileHandler(c *gin.Context) {
	fileName := c.Query("file")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file parameter is required"})
		return
	}

	if err := h.service.UploadFile(c.Request.Context(), fileName); err != nil {
		log.Printf("Error uploading file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "File uploaded successfully"})
}

func (h *HandlerImpl) ProcessBatchHandler(c *gin.Context) {
	if err := h.service.ProcessBatch(c.Request.Context()); err != nil {
		log.Printf("Error processing batch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Batch processing started"})
}

func (h *HandlerImpl) ListFilesHandler(c *gin.Context) {
	files, err := h.service.ListFiles(c.Request.Context())
	if err != nil {
		log.Printf("Error listing files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (h *HandlerImpl) UploadMultipleHandler(c *gin.Context) {
	var request struct {
		Files       []string `json:"files" binding:"required"`
		Concurrency int      `json:"concurrency"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.UploadMultiple(c.Request.Context(), request.Files, request.Concurrency)
	if err != nil {
		log.Printf("Error uploading multiple files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *HandlerImpl) UploadAllHandler(c *gin.Context) {
	var request struct {
		Pattern     string `json:"pattern"`
		Concurrency int    `json:"concurrency"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.UploadAll(c.Request.Context(), request.Pattern, request.Concurrency)
	if err != nil {
		log.Printf("Error uploading all files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
