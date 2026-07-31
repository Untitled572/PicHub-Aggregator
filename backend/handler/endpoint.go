package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pichub/backend/model"
)

var endpointNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func validateEndpointName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", fmt.Errorf("endpoint name is required")
	}
	if !endpointNamePattern.MatchString(name) {
		return "", fmt.Errorf("endpoint name must be lowercase letters, digits or hyphens")
	}
	return name, nil
}

// ListEndpoints GET /api/endpoints 自定义端点列表
// @Summary 自定义端点列表
// @Tags Endpoints
// @Produce json
// @Success 200 {array} model.Endpoint
// @Failure 500 {object} map[string]interface{}
// @Router /api/endpoints [get]
func (h *Handler) ListEndpoints(c *gin.Context) {
	endpoints, err := h.store.ListEndpoints()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, endpoints)
}

// CreateEndpoint POST /api/endpoints 新建自定义端点
// @Summary 新建自定义端点
// @Tags Endpoints
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body EndpointCreateRequest true "端点配置"
// @Success 201 {object} model.Endpoint
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{} "名称已存在"
// @Failure 500 {object} map[string]interface{}
// @Router /api/endpoints [post]
func (h *Handler) CreateEndpoint(c *gin.Context) {
	var body EndpointCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name, err := validateEndpointName(body.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if exists, _ := h.store.EndpointNameExists(name, 0); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "endpoint name already exists"})
		return
	}
	ep := &model.Endpoint{
		Name:      name,
		BoundTags: body.BoundTags,
		Enabled:   true,
	}
	if ep.BoundTags == nil {
		ep.BoundTags = []string{}
	}
	id, err := h.store.CreateEndpoint(ep)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ep.ID = id
	c.JSON(http.StatusCreated, ep)
}

// UpdateEndpoint PUT /api/endpoints/:id 更新自定义端点
// @Summary 更新自定义端点
// @Tags Endpoints
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "端点 ID"
// @Param body body EndpointUpdateRequest true "端点配置"
// @Success 200 {object} model.Endpoint
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{} "名称已存在"
// @Failure 500 {object} map[string]interface{}
// @Router /api/endpoints/{id} [put]
func (h *Handler) UpdateEndpoint(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ep, err := h.store.GetEndpoint(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}
	var body EndpointUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name != nil {
		name, verr := validateEndpointName(*body.Name)
		if verr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": verr.Error()})
			return
		}
		if name != ep.Name {
			if exists, _ := h.store.EndpointNameExists(name, ep.ID); exists {
				c.JSON(http.StatusConflict, gin.H{"error": "endpoint name already exists"})
				return
			}
			ep.Name = name
		}
	}
	if body.BoundTags != nil {
		ep.BoundTags = body.BoundTags
		if ep.BoundTags == nil {
			ep.BoundTags = []string{}
		}
	}
	if body.Enabled != nil {
		ep.Enabled = *body.Enabled
	}
	if err := h.store.UpdateEndpoint(ep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ep)
}

// DeleteEndpoint DELETE /api/endpoints/:id 删除自定义端点
// @Summary 删除自定义端点
// @Tags Endpoints
// @Security BearerAuth
// @Param id path int true "端点 ID"
// @Success 204 "已删除"
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/endpoints/{id} [delete]
func (h *Handler) DeleteEndpoint(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.store.DeleteEndpoint(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ToggleEndpoint POST /api/endpoints/:id/toggle 启停自定义端点
// @Summary 启停自定义端点
// @Tags Endpoints
// @Security BearerAuth
// @Param id path int true "端点 ID"
// @Success 200 {object} model.Endpoint
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/endpoints/{id}/toggle [post]
func (h *Handler) ToggleEndpoint(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ep, err := h.store.GetEndpoint(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}
	ep.Enabled = !ep.Enabled
	if err := h.store.UpdateEndpoint(ep); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ep)
}
