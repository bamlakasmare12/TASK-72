package models

type RegisterRequest struct {
	Username    string  `json:"username" validate:"required"`
	Email       string  `json:"email" validate:"required"`
	Password    string  `json:"password" validate:"required"`
	DisplayName string  `json:"display_name" validate:"required"`
	Role        string  `json:"role" validate:"required"`
	JobFamily   *string `json:"job_family,omitempty"`
	Department  *string `json:"department,omitempty"`
	CostCenter  *string `json:"cost_center,omitempty"`
}

type AssignRoleRequest struct {
	UserID int    `json:"user_id" validate:"required"`
	Role   string `json:"role" validate:"required"`
}

type RegisterResponse struct {
	Message string         `json:"message"`
	User    *UserWithRoles `json:"user"`
}

// ValidRoles returns the set of roles a user can self-select at registration.
func ValidRoles() map[string]bool {
	return map[string]bool{
		"learner":                 true,
		"procurement_specialist":  true,
		"approver":                true,
		"finance_analyst":         true,
		"content_moderator":       true,
	}
}

// AllRoles includes system_admin (admin-assignable only).
func AllRoles() map[string]bool {
	m := ValidRoles()
	m["system_admin"] = true
	return m
}
