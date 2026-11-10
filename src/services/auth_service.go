package services

import (
	"strings"

	"collabpm/config"
	"collabpm/models"
	"collabpm/utils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email,max=191"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

func (s *AuthService) Register(req RegisterRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)
	if email == "" {
		return nil, utils.BadRequest("invalid email")
	}

	var count int64
	if err := s.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return nil, utils.Internal()
	}
	if count > 0 {
		return nil, utils.Conflict("email is already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, utils.Internal()
	}

	user := models.User{
		Name:     strings.TrimSpace(req.Name),
		Email:    email,
		Password: string(hash),
	}
	if user.Name == "" {
		return nil, utils.BadRequest("name cannot be empty")
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, utils.Internal()
	}
	return s.issueTokens(user)
}

func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)

	var user models.User
	err := s.db.Where("email = ?", email).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, utils.Unauthorized("invalid email or password")
	}
	if err != nil {
		return nil, utils.Internal()
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return nil, utils.Unauthorized("invalid email or password")
	}
	return s.issueTokens(user)
}

func (s *AuthService) Refresh(req RefreshRequest) (*AuthResponse, error) {
	claims, err := utils.ParseToken(req.RefreshToken, s.cfg.JWTRefreshSecret)
	if err != nil || claims.TokenType != utils.TokenTypeRefresh {
		return nil, utils.Unauthorized("invalid refresh token")
	}

	var user models.User
	if err := s.db.First(&user, claims.UserID).Error; err != nil {
		return nil, utils.Unauthorized("invalid refresh token")
	}
	return s.issueTokens(user)
}

func (s *AuthService) issueTokens(user models.User) (*AuthResponse, error) {
	access, err := utils.SignAccessToken(user.ID, s.cfg.JWTAccessSecret, s.cfg.AccessExpiry)
	if err != nil {
		return nil, utils.Internal()
	}
	refresh, err := utils.SignRefreshToken(user.ID, s.cfg.JWTRefreshSecret, s.cfg.RefreshExpiry)
	if err != nil {
		return nil, utils.Internal()
	}
	return &AuthResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		User:         toUserResponse(user),
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func toUserResponse(u models.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Avatar:    u.Avatar,
		CreatedAt: u.CreatedAt,
	}
}
