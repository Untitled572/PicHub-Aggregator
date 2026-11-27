package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/service"
	"github.com/pichub/backend/store"
)

var (
	errCuratorPageGone = errors.New("curator page no longer exists")
	errCuratorFeedback = errors.New("curator feedback was not confirmed")
)

var curatorFeedbackHTTP = &http.Client{
	Timeout:       5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func parseFeedbackVote(value string) (int, bool) {
	switch value {
	case "like":
		return 1, true
	case "dislike":
		return -1, true
	case "none":
		return 0, true
	default:
		return 0, false
	}
}

func (h *Handler) HistoryFeedback(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		RequestID string `json:"request_id"`
		Vote      string `json:"vote"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil || !appRequestID.MatchString(input.RequestID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_feedback"})
		return
	}
	vote, ok := parseFeedbackVote(input.Vote)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_vote"})
		return
	}
	h.handleWebFeedback(c, id, 0, input.RequestID, vote)
}

func (h *Handler) legacyImageFeedback(c *gin.Context, vote int) {
	imageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || imageID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_feedback"})
		return
	}
	historyID, err := h.store.LatestImageHistoryID(imageID)
	if err != nil {
		appFeedbackError(c, err)
		return
	}
	current, err := h.applyFeedback(c.Request.Context(), "", rand.Text(), historyID, imageID, "", vote)
	if err != nil {
		appFeedbackError(c, err)
		return
	}
	message := "liked"
	if vote < 0 {
		message = "disliked"
	}
	c.JSON(http.StatusOK, gin.H{"message": message, "vote": store.FeedbackValue(current)})
}

func (h *Handler) handleWebFeedback(c *gin.Context, historyID, imageID int64, requestID string, vote int) {
	current, err := h.applyFeedback(c.Request.Context(), "", requestID, historyID, imageID, "", vote)
	if err != nil {
		appFeedbackError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"vote": store.FeedbackValue(current)})
}

func (h *Handler) applyFeedback(ctx context.Context, key, requestID string, historyID, imageID int64, ref string, vote int) (int, error) {
	if key != "" {
		if err := h.store.CheckAppKey(key); err != nil {
			return 0, err
		}
	}
	h.feedbackMu.Lock()
	defer h.feedbackMu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	target, err := h.store.GetFeedbackTarget(historyID, imageID, ref)
	if err != nil {
		return 0, err
	}
	config, err := h.store.GetCuratorFeedbackConfig()
	if err != nil {
		return 0, err
	}
	identity, err := h.store.GetCuratorImageIdentity(target.ImageID)
	if err != nil {
		return 0, err
	}
	kind, supported := store.FeedbackSupport(config, target.SourceURL, identity)
	if kind == "curator" {
		if !supported {
			return 0, store.ErrFeedbackUnavailable
		}
		if key == "" {
			settings, err := h.store.GetSettings()
			if err != nil {
				return 0, err
			}
			if settings.AdminToken == "" && (!settings.LoginEnabled || settings.AdminUsername == "" || settings.AdminPasswordHash == "") {
				return 0, store.ErrFeedbackUnavailable
			}
		}
		replay, err := h.store.CheckFeedbackReceipt(requestID, target, vote)
		if err != nil {
			return 0, err
		}
		if replay {
			return h.store.CuratorVote(identity)
		}
		if err := putCuratorFeedback(ctx, curatorFeedbackHTTP, config, identity, vote); err != nil {
			return 0, err
		}
		if err := h.store.RecordCuratorFeedback(requestID, target, identity, vote); err != nil {
			return 0, err
		}
		return vote, nil
	}
	if !supported {
		return 0, store.ErrFeedbackImageGone
	}
	if key != "" {
		err = h.store.RecordAppFeedback(key, requestID, imageID, vote, ref)
	} else {
		err = h.store.RecordWebFeedback(requestID, target, vote)
	}
	if err != nil {
		return 0, err
	}
	if target.HistoryID > 0 {
		target, err = h.store.GetFeedbackTarget(target.HistoryID, 0, "")
		return target.Vote, err
	}
	return vote, nil
}

func putCuratorFeedback(ctx context.Context, client *http.Client, config store.CuratorFeedbackConfig, identity *store.CuratorImageIdentity, vote int) error {
	value := store.FeedbackValue(vote)
	if value == "none" {
		value = "neutral"
	}
	payload, err := json.Marshal(struct {
		IllustID  string `json:"illust_id"`
		PageIndex int    `json:"page_index"`
		Value     string `json:"value"`
	}{identity.IllustID, *identity.PageIndex, value})
	if err != nil {
		return errCuratorFeedback
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, config.Address+"/admin/api/image-feedback", bytes.NewReader(payload))
	if err != nil {
		return errCuratorFeedback
	}
	req.Header.Set("Authorization", "Bearer "+config.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return errCuratorFeedback
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errCuratorPageGone
	}
	var result struct {
		Value string `json:"value"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result) != nil || result.Value != value {
		return errCuratorFeedback
	}
	return nil
}

func (h *Handler) feedbackForResult(result *service.Result) (int, bool) {
	if result.ImageID <= 0 || result.DistributionRef == "" {
		return 0, false
	}
	history, err := h.store.GetFeedbackTarget(0, result.ImageID, "")
	if err != nil {
		return 0, false
	}
	config, err := h.store.GetCuratorFeedbackConfig()
	if err != nil {
		return 0, false
	}
	identity, err := h.store.GetCuratorImageIdentity(result.ImageID)
	if err != nil {
		return 0, false
	}
	_, supported := store.FeedbackSupport(config, history.SourceURL, identity)
	if identity != nil && identity.PageIndex != nil {
		vote, err := h.store.CuratorVote(identity)
		return vote, supported && err == nil
	}
	return 0, supported
}
