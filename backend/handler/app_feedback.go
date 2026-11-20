package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/store"
)

const appKeyPrefix = "ph_app_"

var appRequestID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var distributionRef = regexp.MustCompile(`^[A-Z2-7]{26}$`)

func appKey(c *gin.Context) string {
	key, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || !strings.HasPrefix(key, appKeyPrefix) || len(key) != len(appKeyPrefix)+43 {
		return ""
	}
	return key
}

func appFeedbackError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidAppKey):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_app_key"})
	case errors.Is(err, store.ErrFeedbackConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "request_id_conflict"})
	case errors.Is(err, store.ErrFeedbackImageGone):
		c.JSON(http.StatusGone, gin.H{"error": "feedback_image_expired"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "app_feedback_failed"})
	}
}

func (h *Handler) GetAppKeyState(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	configured, err := h.store.HasAppKey()
	if err != nil {
		appFeedbackError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"configured": configured})
}

func (h *Handler) GenerateAppKey(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		appFeedbackError(c, err)
		return
	}
	key := appKeyPrefix + base64.RawURLEncoding.EncodeToString(secret[:])
	if err := h.store.SetAppKey(key); err != nil {
		appFeedbackError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"key": key})
}

func (h *Handler) RevokeAppKey(c *gin.Context) {
	if err := h.store.SetAppKey(""); err != nil {
		appFeedbackError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) requireAppKey(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	key := appKey(c)
	if key == "" {
		appFeedbackError(c, store.ErrInvalidAppKey)
		return false
	}
	if err := h.store.CheckAppKey(key); err != nil {
		appFeedbackError(c, err)
		return false
	}
	return true
}

func (h *Handler) CheckAppFeedbackAuth(c *gin.Context) {
	if h.requireAppKey(c) {
		c.Status(http.StatusNoContent)
	}
}

func (h *Handler) ListAppTags(c *gin.Context) {
	if !h.requireAppKey(c) {
		return
	}
	tags, err := h.store.GetTags()
	if err != nil {
		appFeedbackError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func (h *Handler) ListAppEndpoints(c *gin.Context) {
	if !h.requireAppKey(c) {
		return
	}
	endpoints, err := h.store.ListEndpoints()
	if err != nil {
		appFeedbackError(c, err)
		return
	}
	type endpoint struct {
		Name      string   `json:"name"`
		BoundTags []string `json:"bound_tags"`
	}
	enabled := make([]endpoint, 0)
	for _, ep := range endpoints {
		if ep.Enabled {
			enabled = append(enabled, endpoint{Name: ep.Name, BoundTags: ep.BoundTags})
		}
	}
	c.JSON(http.StatusOK, enabled)
}

func (h *Handler) AppFeedback(c *gin.Context) {
	key := appKey(c)
	if key == "" {
		appFeedbackError(c, store.ErrInvalidAppKey)
		return
	}
	var input struct {
		RequestID string `json:"request_id"`
		ImageID   int64  `json:"image_id"`
		Vote      string `json:"vote"`
		Ref       string `json:"distribution_ref"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	if err := c.ShouldBindJSON(&input); err != nil || input.ImageID <= 0 || !appRequestID.MatchString(input.RequestID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_feedback"})
		return
	}
	if input.Ref != "" && !distributionRef.MatchString(input.Ref) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_distribution_ref"})
		return
	}
	var delta int
	switch input.Vote {
	case "like":
		delta = 1
	case "dislike":
		delta = -1
	case "none":
		if input.Ref == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "distribution_ref_required"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_vote"})
		return
	}
	if err := h.store.RecordAppFeedback(key, input.RequestID, input.ImageID, delta, input.Ref); err != nil {
		appFeedbackError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
