package models

import "time"

type LearningPath struct {
	ID              int             `json:"id"`
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Description     *string         `json:"description,omitempty"`
	CategoryID      *int            `json:"category_id,omitempty"`
	CategoryName    string          `json:"category_name,omitempty"`
	TargetJobFamily *string         `json:"target_job_family,omitempty"`
	RequiredCount   int             `json:"required_count"`
	ElectiveMin     int             `json:"elective_min"`
	EstimatedHours  *float64        `json:"estimated_hours,omitempty"`
	Difficulty      *string         `json:"difficulty,omitempty"`
	IsActive        bool            `json:"is_active"`
	Items           []PathItem      `json:"items,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type PathItem struct {
	ID           int       `json:"id"`
	PathID       int       `json:"path_id"`
	ResourceID   int       `json:"resource_id"`
	ItemType     string    `json:"item_type"` // required, elective
	SortOrder    int       `json:"sort_order"`
	Resource     *Resource `json:"resource,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserEnrollment struct {
	ID          int        `json:"id"`
	UserID      int        `json:"user_id"`
	PathID      int        `json:"path_id"`
	Status      string     `json:"status"`
	EnrolledAt  time.Time  `json:"enrolled_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	LastAccessed time.Time `json:"last_accessed"`
	Path        *LearningPath `json:"path,omitempty"`
}

type UserProgress struct {
	ID           int        `json:"id"`
	UserID       int        `json:"user_id"`
	ResourceID   int        `json:"resource_id"`
	PathID       *int       `json:"path_id,omitempty"`
	Status       string     `json:"status"`
	ProgressPct  int        `json:"progress_pct"`
	TimeSpentMin int        `json:"time_spent_mins"`
	LastPosition *string    `json:"last_position,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	SyncedAt     time.Time  `json:"synced_at"`
	Resource     *Resource  `json:"resource,omitempty"`
}

type EnrollmentDetail struct {
	Enrollment       UserEnrollment `json:"enrollment"`
	Path             LearningPath   `json:"path"`
	RequiredItems    []PathItemProgress `json:"required_items"`
	ElectiveItems    []PathItemProgress `json:"elective_items"`
	RequiredComplete int    `json:"required_complete"`
	ElectiveComplete int    `json:"elective_complete"`
	IsPathComplete   bool   `json:"is_path_complete"`
}

type PathItemProgress struct {
	Item     PathItem      `json:"item"`
	Progress *UserProgress `json:"progress,omitempty"`
}

type UpdateProgressRequest struct {
	ResourceID   int    `json:"resource_id" validate:"required"`
	PathID       *int   `json:"path_id,omitempty"`
	Status       string `json:"status"`
	ProgressPct  int    `json:"progress_pct"`
	TimeSpentMin int    `json:"time_spent_mins"`
	LastPosition string `json:"last_position"`
}

type Recommendation struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	ResourceID int       `json:"resource_id"`
	Score      float64   `json:"score"`
	Reason     string    `json:"reason"`
	CategoryID *int      `json:"category_id,omitempty"`
	Resource   *Resource `json:"resource,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type LearningRecordCSV struct {
	Username     string `json:"username"`
	ResourceTitle string `json:"resource_title"`
	PathTitle    string `json:"path_title"`
	Status       string `json:"status"`
	ProgressPct  int    `json:"progress_pct"`
	TimeSpentMin int    `json:"time_spent_mins"`
	StartedAt    string `json:"started_at"`
	CompletedAt  string `json:"completed_at"`
}
