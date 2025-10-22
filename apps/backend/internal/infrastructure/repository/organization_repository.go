package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// OrganizationRepository implements domain.OrganizationRepository
type OrganizationRepository struct {
	db *sql.DB
}

// NewOrganizationRepository creates a new organization repository
func NewOrganizationRepository(db *sql.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

// Create creates a new organization
func (r *OrganizationRepository) Create(org *domain.Organization) error {
	query := `
		INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	now := time.Now()
	org.ID = uuid.New()
	org.CreatedAt = now
	org.UpdatedAt = now

	_, err := r.db.Exec(query,
		org.ID,
		org.Name,
		org.Domain,
		org.PlanType,
		org.MaxAgents,
		org.MaxUsers,
		org.IsActive,
		org.CreatedAt,
		org.UpdatedAt,
	)

	return err
}

// GetByID retrieves an organization by ID
func (r *OrganizationRepository) GetByID(id uuid.UUID) (*domain.Organization, error) {
	query := `
		SELECT id, name, domain, plan_type, max_agents, max_users, is_active, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`

	org := &domain.Organization{}
	err := r.db.QueryRow(query, id).Scan(
		&org.ID,
		&org.Name,
		&org.Domain,
		&org.PlanType,
		&org.MaxAgents,
		&org.MaxUsers,
		&org.IsActive,
		&org.CreatedAt,
		&org.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("organization not found")
	}
	if err != nil {
		return nil, err
	}

	return org, nil
}

// GetByDomain retrieves an organization by domain
func (r *OrganizationRepository) GetByDomain(domainName string) (*domain.Organization, error) {
	query := `
		SELECT id, name, domain, plan_type, max_agents, max_users, is_active, created_at, updated_at
		FROM organizations
		WHERE domain = $1
	`

	org := &domain.Organization{}
	err := r.db.QueryRow(query, domainName).Scan(
		&org.ID,
		&org.Name,
		&org.Domain,
		&org.PlanType,
		&org.MaxAgents,
		&org.MaxUsers,
		&org.IsActive,
		&org.CreatedAt,
		&org.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Organization doesn't exist yet
	}
	if err != nil {
		return nil, err
	}

	return org, nil
}

// Update updates an organization
func (r *OrganizationRepository) Update(org *domain.Organization) error {
	query := `
		UPDATE organizations
		SET name = $1, plan_type = $2, max_agents = $3, max_users = $4, is_active = $5, updated_at = $6
		WHERE id = $7
	`

	org.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		org.Name,
		org.PlanType,
		org.MaxAgents,
		org.MaxUsers,
		org.IsActive,
		org.UpdatedAt,
		org.ID,
	)

	return err
}

// Delete deletes an organization
func (r *OrganizationRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM organizations WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}
