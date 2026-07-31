package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetStats(c *gin.Context) {
	todayStr := time.Now().Format("2006-01-02")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	rangeParam := c.Query("range")

	if rangeParam != "" {
		switch rangeParam {
		case "today":
			startDate = todayStr
			endDate = todayStr
		case "7d":
			startDate = time.Now().AddDate(0, 0, -6).Format("2006-01-02")
			endDate = todayStr
		case "30d":
			startDate = time.Now().AddDate(0, 0, -29).Format("2006-01-02")
			endDate = todayStr
		case "all":
			startDate = "1970-01-01"
			endDate = "2099-12-31"
		}
	}

	if startDate == "" {
		startDate = todayStr
	}
	if endDate == "" {
		endDate = todayStr
	}

	overview, err := h.store.GetStatsRange(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	total, err := h.store.GetTotalRequests()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"today":      overview,
		"start_date": startDate,
		"end_date":   endDate,
		"range":      rangeParam,
		"total":      gin.H{"total_requests": total},
	})
}

func (h *Handler) GetImageHistory(c *gin.Context) {
	limit := 20
	offset := 0
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	if o, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && o >= 0 {
		offset = o
	}

	history, total, err := h.store.GetImageHistory(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
