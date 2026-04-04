package programs

import (
	"context"

	"lms/internal/model"
)

type Repository interface {
	Create(ctx context.Context, p *model.Program) error
	GetByID(ctx context.Context, id, branchID string) (*model.Program, error)
	Update(ctx context.Context, p *model.Program) error
	UpdateStatus(ctx context.Context, id, branchID, status string) error
	List(ctx context.Context, branchID string, f ProgramFilter, p model.Pagination) (model.PageResult[*model.Program], error)
	GetPrerequisites(ctx context.Context, programID string) ([]*model.ProgramPrerequisite, error)
	AddPrerequisite(ctx context.Context, pr *model.ProgramPrerequisite) error
	RemovePrerequisite(ctx context.Context, programID, requiredProgramID string) error
	GetEnrollmentRules(ctx context.Context, programID string) ([]*model.EnrollmentRule, error)
	AddEnrollmentRule(ctx context.Context, rule *model.EnrollmentRule) error
	RemoveEnrollmentRule(ctx context.Context, programID, ruleID string) error
}

type ProgramFilter struct {
	Status   *string
	Category *string
	Search   *string
}
