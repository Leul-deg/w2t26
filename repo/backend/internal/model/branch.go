package model

import "time"

// Branch is a physical library location. Every resource record is branch-scoped.
type Branch struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Address   *string   `json:"address,omitempty"`
	Phone     *string   `json:"phone,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserBranchAssignment links a staff user to a branch.
// Administrators bypass this table and have implicit all-branch access.
type UserBranchAssignment struct {
	UserID     string    `json:"user_id"`
	BranchID   string    `json:"branch_id"`
	AssignedAt time.Time `json:"assigned_at"`
	AssignedBy *string   `json:"assigned_by,omitempty"`
}
