package handler

import (
	"github.com/pichub/backend/service"
	"github.com/pichub/backend/store"
)

type Handler struct {
	store         *store.Store
	engine        *service.Engine
	healthChecker *service.HealthChecker
}

func NewHandler(st *store.Store, eng *service.Engine) *Handler {
	return &Handler{store: st, engine: eng}
}

func NewHandlerWithEngine(st *store.Store, eng *service.Engine, hc *service.HealthChecker) *Handler {
	return &Handler{store: st, engine: eng, healthChecker: hc}
}
