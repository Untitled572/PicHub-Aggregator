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

func (h *Handler) ListEndpoints(c *gin.Context) {
	endpoints, err := h.store.ListEndpoints()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, endpoints)
}

func (h *Handler) CreateEndpoint(c *gin.Context) {
	var body struct {
		Name      string   `json:"name"`
		BoundTags []string `json:"bound_tags"`
	}
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
	var body struct {
		Name      *string  `json:"name"`
		BoundTags []string `json:"bound_tags"`
		Enabled   *bool    `json:"enabled"`
	}
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
