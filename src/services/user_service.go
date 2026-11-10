package services

import (
	"strings"
	"time"

	"collabpm/models"
	"collabpm/utils"

	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

type UserResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateProfileRequest struct {
	Name   *string `json:"name" binding:"omitempty,min=1,max=100"`
	Avatar *string `json:"avatar" binding:"omitempty,max=500"`
}

func (s *UserService) GetMe(userID uint) (*UserResponse, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, utils.NotFound("user not found")
	}
	resp := toUserResponse(user)
	return &resp, nil
}

func (s *UserService) UpdateMe(userID uint, req UpdateProfileRequest) (*UserResponse, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, utils.NotFound("user not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, utils.BadRequest("name cannot be empty")
		}
		user.Name = name
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}

	if err := s.db.Save(&user).Error; err != nil {
		return nil, utils.Internal()
	}
	resp := toUserResponse(user)
	return &resp, nil
}

// Search powers the member picker: fuzzy match on name or email.
func (s *UserService) Search(query string) ([]UserResponse, error) {
	q := strings.TrimSpace(query)
	like := "%" + q + "%"

	var users []models.User
	err := s.db.
		Where("name LIKE ? OR email LIKE ?", like, like).
		Order("name ASC").
		Limit(20).
		Find(&users).Error
	if err != nil {
		return nil, utils.Internal()
	}

	result := make([]UserResponse, 0, len(users))
	for _, u := range users {
		result = append(result, toUserResponse(u))
	}
	return result, nil
}
