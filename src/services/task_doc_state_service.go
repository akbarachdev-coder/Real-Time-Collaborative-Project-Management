package services

import (
	"encoding/base64"
	"time"

	"collabpm/models"
	"collabpm/utils"

	"gorm.io/gorm"
)

type TaskDocStateService struct {
	db *gorm.DB
}

func NewTaskDocStateService(db *gorm.DB) *TaskDocStateService {
	return &TaskDocStateService{db: db}
}

type TaskDocStateResponse struct {
	TaskID       uint      `json:"task_id"`
	CRDTSnapshot string    `json:"crdt_snapshot"`
	ClockVersion int       `json:"clock_version"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PutTaskDocStateRequest struct {
	CRDTSnapshot string `json:"crdt_snapshot" binding:"required"`
	ClockVersion int    `json:"clock_version" binding:"min=1"`
}

func (s *TaskDocStateService) Get(taskID, userID uint) (*TaskDocStateResponse, error) {
	if _, err := s.loadTaskAccess(taskID, userID); err != nil {
		return nil, err
	}

	var state models.TaskDocState
	err := s.db.First(&state, "task_id = ?", taskID).Error
	if err == gorm.ErrRecordNotFound {
		return &TaskDocStateResponse{
			TaskID:       taskID,
			CRDTSnapshot: "",
			ClockVersion: 0,
		}, nil
	}
	if err != nil {
		return nil, utils.Internal()
	}

	return &TaskDocStateResponse{
		TaskID:       state.TaskID,
		CRDTSnapshot: base64.StdEncoding.EncodeToString(state.CRDTSnapshot),
		ClockVersion: state.ClockVersion,
		UpdatedAt:    state.UpdatedAt,
	}, nil
}

func (s *TaskDocStateService) Put(taskID, userID uint, req PutTaskDocStateRequest) (*TaskDocStateResponse, error) {
	if _, err := s.loadTaskAccess(taskID, userID); err != nil {
		return nil, err
	}

	raw, err := base64.StdEncoding.DecodeString(req.CRDTSnapshot)
	if err != nil {
		return nil, utils.BadRequest("crdt_snapshot must be valid base64")
	}

	var existing models.TaskDocState
	err = s.db.First(&existing, "task_id = ?", taskID).Error
	switch {
	case err == gorm.ErrRecordNotFound:
		state := models.TaskDocState{
			TaskID:       taskID,
			CRDTSnapshot: raw,
			ClockVersion: req.ClockVersion,
		}
		if err := s.db.Create(&state).Error; err != nil {
			return nil, utils.Internal()
		}
		return &TaskDocStateResponse{
			TaskID:       state.TaskID,
			CRDTSnapshot: req.CRDTSnapshot,
			ClockVersion: state.ClockVersion,
			UpdatedAt:    state.UpdatedAt,
		}, nil
	case err != nil:
		return nil, utils.Internal()
	}

	if req.ClockVersion < existing.ClockVersion {
		return nil, utils.Conflict("clock_version must not go backwards")
	}
	if req.ClockVersion == existing.ClockVersion && string(existing.CRDTSnapshot) == string(raw) {
		return &TaskDocStateResponse{
			TaskID:       existing.TaskID,
			CRDTSnapshot: req.CRDTSnapshot,
			ClockVersion: existing.ClockVersion,
			UpdatedAt:    existing.UpdatedAt,
		}, nil
	}

	existing.CRDTSnapshot = raw
	existing.ClockVersion = req.ClockVersion
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, utils.Internal()
	}
	return &TaskDocStateResponse{
		TaskID:       existing.TaskID,
		CRDTSnapshot: req.CRDTSnapshot,
		ClockVersion: existing.ClockVersion,
		UpdatedAt:    existing.UpdatedAt,
	}, nil
}

func (s *TaskDocStateService) loadTaskAccess(taskID, userID uint) (*models.Task, error) {
	var task models.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, utils.NotFound("task not found")
	}
	if _, err := workspaceIDForProject(s.db, task.ProjectID, userID); err != nil {
		return nil, err
	}
	return &task, nil
}
