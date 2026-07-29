package http

import (
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/google/uuid"
)

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	FullName string `json:"full_name" binding:"required"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type userResponse struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u domain.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Username:  u.Username,
		FullName:  u.FullName,
		CreatedAt: u.CreatedAt,
	}
}

type loginResponse struct {
	User  userResponse `json:"user"`
	Token string       `json:"token"`
}
