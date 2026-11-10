package models

import "time"

const (
	RoleOwner  = "OWNER"
	RoleAdmin  = "ADMIN"
	RoleMember = "MEMBER"
	RoleGuest  = "GUEST"
)

var roleRank = map[string]int{
	RoleGuest:  1,
	RoleMember: 2,
	RoleAdmin:  3,
	RoleOwner:  4,
}

func IsValidRole(role string) bool {
	_, ok := roleRank[role]
	return ok
}

// RoleAtLeast reports whether a has at least the rank of min.
func RoleAtLeast(a, min string) bool {
	return roleRank[a] >= roleRank[min]
}

type Workspace struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Slug      string    `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	OwnerID   uint      `gorm:"index;not null" json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkspaceMember struct {
	WorkspaceID uint      `gorm:"primaryKey" json:"workspace_id"`
	UserID      uint      `gorm:"primaryKey" json:"user_id"`
	Role        string    `gorm:"size:10;not null" json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}
