package handlers

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// UploadHandler handles file uploads
type UploadHandler struct {
	ipfsClient *ipfs.Client
	logger     *logger.Logger
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(ipfsClient *ipfs.Client, logger *logger.Logger) *UploadHandler {
	return &UploadHandler{
		ipfsClient: ipfsClient,
		logger:     logger.WithComponent("upload-handler"),
	}
}

// UploadImage handles image uploads to IPFS
func (h *UploadHandler) UploadImage(c *gin.Context) {
	// Check if IPFS client is available and healthy
	if h.ipfsClient == nil {
		response.InternalServerError(c, "IPFS service not configured")
		return
	}

	if !h.ipfsClient.IsHealthy(c.Request.Context()) {
		h.logger.Warn("IPFS daemon not available for image upload")
		response.InternalServerError(c, "IPFS daemon not running. Please start IPFS with: ipfs daemon")
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		response.BadRequest(c, "Image file is required")
		return
	}
	defer file.Close()

	// Validate file type (simple check)
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		response.BadRequest(c, "Invalid image format. Allowed: jpg, jpeg, png, gif, webp")
		return
	}

	// Validate file size (max 10MB)
	if header.Size > 10*1024*1024 {
		response.BadRequest(c, "Image too large. Maximum size: 10MB")
		return
	}

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		h.logger.Error("Failed to read image file", "error", err)
		response.InternalServerError(c, "Failed to process image")
		return
	}

	// Upload to IPFS
	cid, err := h.ipfsClient.Add(c.Request.Context(), data)
	if err != nil {
		h.logger.Error("Failed to upload image to IPFS", "error", err)
		response.InternalServerError(c, "Failed to upload to IPFS. Is the daemon running?")
		return
	}

	h.logger.Info("Image uploaded to IPFS", "cid", cid, "size", len(data), "filename", header.Filename)

	// Return IPFS URL (assuming a public gateway or local gateway for viewing)
	// For now, we return the CID and a gateway URL structure
	// In a real P2P app, the frontend might load from local IPFS gateway
	url := fmt.Sprintf("https://ipfs.io/ipfs/%s", cid)

	response.Success(c, gin.H{
		"cid": cid,
		"url": url,
	})
}
