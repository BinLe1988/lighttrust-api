package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Allowed image MIME types
var allowedImageTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"image/bmp":       ".bmp",
	"image/svg+xml":   ".svg",
}

// UploadImage handles image file upload for playground
func UploadImage(c *gin.Context) {
	// Check upload permission
	userId := c.GetInt("id")
	userRole := c.GetInt("role")
	if userRole < common.ImageUploadPermission {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "No permission to upload images",
		})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Failed to read uploaded file: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Validate file size (max 10MB)
	maxSize := int64(10 * 1024 * 1024)
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("File too large, max %dMB", maxSize/(1024*1024)),
		})
		return
	}

	// Detect content type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Failed to read file content",
		})
		return
	}
	contentType := http.DetectContentType(buffer)

	// Validate image type
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Unsupported file type: %s. Allowed: JPEG, PNG, GIF, WebP, BMP, SVG", contentType),
		})
		return
	}

	// Reset file reader to beginning
	file.Seek(0, io.SeekStart)

	// Generate unique filename
	filename := fmt.Sprintf("%d_%s%s", userId, uuid.New().String()[:8], ext)

	// Create date-based subdirectory
	dateDir := time.Now().Format("2006/01/02")
	uploadDir := filepath.Join("web", "default", "dist", "uploads", dateDir)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create upload directory",
		})
		return
	}

	// Save file
	destPath := filepath.Join(uploadDir, filename)
	destFile, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save file",
		})
		return
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to write file",
		})
		return
	}

	// Build accessible URL
	// Remove "web/default/dist" prefix for URL path
	urlPath := strings.TrimPrefix(destPath, "web/default/dist/")
	imageURL := fmt.Sprintf("/uploads/%s/%s", dateDir, filename)

	common.SysLog(fmt.Sprintf("User %d uploaded image: %s (%d bytes)", userId, filename, header.Size))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"url":      imageURL,
			"filename": filename,
			"size":     header.Size,
			"type":     contentType,
		},
	})

	// Suppress unused variable warning
	_ = urlPath
}
