package handler

import (
	"github.com/pichub/backend/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(st *store.Store) *Handler {
	return &Handler{store: st}
}
