package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Monthlyaway/short-link/internal/service"
	"github.com/gin-gonic/gin"
)

// URLHandler handles HTTP requests for URL operations
type URLHandler struct {
	service *service.URLService
	baseURL string
}

// NewURLHandler creates a new URL handler instance
func NewURLHandler(service *service.URLService, baseURL string) *URLHandler {
	return &URLHandler{
		service: service,
		baseURL: baseURL,
	}
}

// CreateShortURLRequest represents the request body for creating a short URL
type CreateShortURLRequest struct {
	URL       string     `json:"url" binding:"required"`
	ExpiredAt *time.Time `json:"expired_at,omitempty"`
}

// CreateShortURLResponse represents the response for creating a short URL
type CreateShortURLResponse struct {
	ShortCode   string     `json:"short_code"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ExpiredAt   *time.Time `json:"expired_at,omitempty"`
}

// URLInfoResponse represents the response for URL info
type URLInfoResponse struct {
	ShortCode   string     `json:"short_code"`
	OriginalURL string     `json:"original_url"`
	VisitCount  uint64     `json:"visit_count"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiredAt   *time.Time `json:"expired_at,omitempty"`
}

// Response represents a generic API response
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// CreateShortURL handles POST /api/v1/shorten
func (h *URLHandler) CreateShortURL(c *gin.Context) {
	var req CreateShortURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	mapping, err := h.service.CreateShortURL(c.Request.Context(), req.URL, req.ExpiredAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create short URL: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: CreateShortURLResponse{
			ShortCode:   mapping.ShortCode,
			ShortURL:    h.buildShortURL(mapping.ShortCode),
			OriginalURL: mapping.OriginalURL,
			ExpiredAt:   mapping.ExpiredAt,
		},
	})
}

// RedirectToOriginalURL handles GET /{short_code}
func (h *URLHandler) RedirectToOriginalURL(c *gin.Context) {
	shortCode := c.Param("short_code")
	if shortCode == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "Short code is required",
		})
		return
	}

	originalURL, err := h.service.GetOriginalURL(c.Request.Context(), shortCode)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    http.StatusNotFound,
			Message: "Short URL not found or expired",
		})
		return
	}

	// Record visit asynchronously
	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	go h.service.RecordVisit(c.Request.Context(), shortCode, ip, userAgent)

	// Redirect to original URL
	c.Redirect(http.StatusFound, originalURL)
}

// GetURLInfo handles GET /api/v1/info/{short_code}
func (h *URLHandler) GetURLInfo(c *gin.Context) {
	shortCode := c.Param("short_code")
	if shortCode == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "Short code is required",
		})
		return
	}

	mapping, err := h.service.GetURLInfo(c.Request.Context(), shortCode)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    http.StatusNotFound,
			Message: "Short URL not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: URLInfoResponse{
			ShortCode:   mapping.ShortCode,
			OriginalURL: mapping.OriginalURL,
			VisitCount:  mapping.VisitCount,
			CreatedAt:   mapping.CreatedAt,
			ExpiredAt:   mapping.ExpiredAt,
		},
	})
}

// HealthCheck handles GET /health
func (h *URLHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "OK",
	})
}

// buildShortURL builds the full short URL
func (h *URLHandler) buildShortURL(shortCode string) string {
	return fmt.Sprintf("%s/%s", h.baseURL, shortCode)
}
