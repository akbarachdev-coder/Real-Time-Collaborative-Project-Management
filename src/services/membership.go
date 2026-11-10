package services

import (
	"collabpm/models"
	"collabpm/utils"

	"gorm.io/gorm"
)

func IsProjectMember(db *gorm.DB, projectID, userID uint) (bool, error) {
	var count int64
	err := db.Model(&models.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func RequireProjectMember(db *gorm.DB, projectID, userID uint) error {
	ok, err := IsProjectMember(db, projectID, userID)
	if err != nil {
		return utils.Internal()
	}
	if !ok {
		return utils.NotFound("project not found")
	}
	return nil
}

func RequireProjectOwner(db *gorm.DB, projectID, userID uint) error {
	var member models.ProjectMember
	err := db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&member).Error
	if err == gorm.ErrRecordNotFound {
		return utils.NotFound("project not found")
	}
	if err != nil {
		return utils.Internal()
	}
	if member.Role != "owner" {
		return utils.Forbidden("only the project owner can perform this action")
	}
	return nil
}
