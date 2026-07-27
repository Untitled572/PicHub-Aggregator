package handler

import (
	"github.com/pichub/backend/service"
	"github.com/pichub/backend/store"
)

type Handler struct {
	store         *store.Store
	healthChecker *service.HealthChecker
}

func NewHandler(st *store.Store) *Handler {
	return &Handler{store: st}
}

func NewHandlerWithHealth(st *store.Store, hc *service.HealthChecker) *Handler {
	return &Handler{store: st, healthChecker: hc}
}
