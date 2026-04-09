package services

import (
	"context"
	"fmt"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
)

type LearningService struct {
	learningRepo *repository.LearningRepository
	searchRepo   *repository.SearchRepository
}

func NewLearningService(learningRepo *repository.LearningRepository, searchRepo *repository.SearchRepository) *LearningService {
	return &LearningService{
		learningRepo: learningRepo,
		searchRepo:   searchRepo,
	}
}

func (s *LearningService) GetAllPaths(ctx context.Context) ([]models.LearningPath, error) {
	return s.learningRepo.GetAllPaths(ctx)
}

func (s *LearningService) GetPathDetail(ctx context.Context, pathID int) (*models.LearningPath, error) {
	return s.learningRepo.GetPathByID(ctx, pathID)
}

func (s *LearningService) Enroll(ctx context.Context, userID, pathID int) (*models.UserEnrollment, error) {
	path, err := s.learningRepo.GetPathByID(ctx, pathID)
	if err != nil || path == nil {
		return nil, fmt.Errorf("learning path not found")
	}
	if !path.IsActive {
		return nil, fmt.Errorf("learning path is not active")
	}

	enrollment, err := s.learningRepo.Enroll(ctx, userID, pathID)
	if err != nil {
		return nil, fmt.Errorf("failed to enroll: %w", err)
	}

	// Initialize progress records for all items in the path
	for _, item := range path.Items {
		req := models.UpdateProgressRequest{
			ResourceID:  item.ResourceID,
			PathID:      &pathID,
			Status:      "not_started",
			ProgressPct: 0,
		}
		_, _ = s.learningRepo.UpsertProgress(ctx, userID, req)
	}

	return enrollment, nil
}

func (s *LearningService) DropEnrollment(ctx context.Context, userID, pathID int) error {
	return s.learningRepo.DropEnrollment(ctx, userID, pathID)
}

func (s *LearningService) GetEnrollments(ctx context.Context, userID int) ([]models.UserEnrollment, error) {
	return s.learningRepo.GetUserEnrollments(ctx, userID)
}

// GetEnrollmentDetail returns a detailed view of a user's enrollment including
// progress for each required and elective item and completion validation.
func (s *LearningService) GetEnrollmentDetail(ctx context.Context, userID, pathID int) (*models.EnrollmentDetail, error) {
	enrollments, err := s.learningRepo.GetUserEnrollments(ctx, userID)
	if err != nil {
		return nil, err
	}
	var enrollment *models.UserEnrollment
	for _, e := range enrollments {
		if e.PathID == pathID {
			enrollment = &e
			break
		}
	}
	if enrollment == nil {
		return nil, fmt.Errorf("enrollment not found")
	}

	path, err := s.learningRepo.GetPathByID(ctx, pathID)
	if err != nil || path == nil {
		return nil, fmt.Errorf("learning path not found")
	}

	progress, err := s.learningRepo.GetUserProgress(ctx, userID, &pathID)
	if err != nil {
		return nil, err
	}

	// Build progress lookup
	progressMap := make(map[int]*models.UserProgress)
	for i := range progress {
		progressMap[progress[i].ResourceID] = &progress[i]
	}

	var requiredItems, electiveItems []models.PathItemProgress
	var requiredComplete, electiveComplete int

	for _, item := range path.Items {
		pip := models.PathItemProgress{
			Item:     item,
			Progress: progressMap[item.ResourceID],
		}
		if item.ItemType == "required" {
			requiredItems = append(requiredItems, pip)
			if pip.Progress != nil && pip.Progress.Status == "completed" {
				requiredComplete++
			}
		} else {
			electiveItems = append(electiveItems, pip)
			if pip.Progress != nil && pip.Progress.Status == "completed" {
				electiveComplete++
			}
		}
	}

	// Validate path completion: all required done AND elective_min met
	isComplete := requiredComplete >= path.RequiredCount && electiveComplete >= path.ElectiveMin

	return &models.EnrollmentDetail{
		Enrollment:       *enrollment,
		Path:             *path,
		RequiredItems:    requiredItems,
		ElectiveItems:    electiveItems,
		RequiredComplete: requiredComplete,
		ElectiveComplete: electiveComplete,
		IsPathComplete:   isComplete,
	}, nil
}

// UpdateProgress updates a user's progress on a resource and checks path completion.
func (s *LearningService) UpdateProgress(ctx context.Context, userID int, req models.UpdateProgressRequest) (*models.UserProgress, error) {
	p, err := s.learningRepo.UpsertProgress(ctx, userID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update progress: %w", err)
	}

	// Record completion event
	if req.Status == "completed" {
		_ = s.searchRepo.RecordEvent(ctx, userID, req.ResourceID, "complete")

		// Increment resource completion count
		// (view count is handled by search service)
	}

	// Check if this completes the learning path
	if req.PathID != nil && req.Status == "completed" {
		path, err := s.learningRepo.GetPathByID(ctx, *req.PathID)
		if err == nil && path != nil {
			reqDone, elecDone, err := s.learningRepo.GetProgressCountForPath(ctx, userID, *req.PathID)
			if err == nil {
				if reqDone >= path.RequiredCount && elecDone >= path.ElectiveMin {
					_ = s.learningRepo.CompleteEnrollment(ctx, userID, *req.PathID)
				}
			}
		}
	}

	return p, nil
}

func (s *LearningService) GetProgress(ctx context.Context, userID int, pathID *int) ([]models.UserProgress, error) {
	return s.learningRepo.GetUserProgress(ctx, userID, pathID)
}

func (s *LearningService) GetLearningRecords(ctx context.Context, userID int) ([]models.LearningRecordCSV, error) {
	return s.learningRepo.GetLearningRecords(ctx, userID)
}

func (s *LearningService) GetRecommendations(ctx context.Context, userID int, limit int) ([]models.Recommendation, error) {
	return s.learningRepo.GetRecommendations(ctx, userID, limit)
}
