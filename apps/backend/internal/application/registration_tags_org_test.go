package application

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Registering an agent or an MCP server takes a tagIds list. An entry that is
// a tag id is attached only when that tag belongs to the caller's
// organization: a tag id of another organization, or an id that names no tag,
// is skipped, so the other organization's tag key, value and description are
// never returned with the caller's agent or server.

// registrationTagRepo is an in-memory TagRepository holding the tags of two
// organizations. It records every AddTagsToAgent call.
type registrationTagRepo struct {
	domain.TagRepository
	tags      map[uuid.UUID]*domain.Tag
	agentTags map[uuid.UUID][]uuid.UUID
}

func newRegistrationTagRepo(tags ...*domain.Tag) *registrationTagRepo {
	r := &registrationTagRepo{
		tags:      make(map[uuid.UUID]*domain.Tag),
		agentTags: make(map[uuid.UUID][]uuid.UUID),
	}
	for _, t := range tags {
		r.tags[t.ID] = t
	}
	return r
}

func (r *registrationTagRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	if t, ok := r.tags[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("tag not found")
}

func (r *registrationTagRepo) SearchTags(ctx context.Context, organizationID uuid.UUID, query string, category *domain.TagCategory) ([]*domain.Tag, error) {
	found := make([]*domain.Tag, 0)
	for _, t := range r.tags {
		if t.OrganizationID == organizationID && t.Key == query {
			found = append(found, t)
		}
	}
	return found, nil
}

func (r *registrationTagRepo) AddTagsToAgent(ctx context.Context, agentID uuid.UUID, tagIDs []uuid.UUID) error {
	r.agentTags[agentID] = append(r.agentTags[agentID], tagIDs...)
	return nil
}

func registrationTag(orgID uuid.UUID, key string) *domain.Tag {
	return &domain.Tag{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Key:            key,
		Value:          key,
		Category:       domain.TagCategoryCustom,
		Description:    key + " description",
		Color:          "#6b7280",
		CreatedAt:      time.Now().UTC(),
		CreatedBy:      uuid.New(),
	}
}

func TestApplyRegistrationTags_SkipsATagIDOfAnotherOrganization(t *testing.T) {
	callerOrg, otherOrg := uuid.New(), uuid.New()
	ownTag := registrationTag(callerOrg, "team")
	otherTag := registrationTag(otherOrg, "secret-project")
	repo := newRegistrationTagRepo(ownTag, otherTag)
	service := &AgentService{tagRepo: repo}
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: callerOrg, Name: "caller-agent"}

	service.applyRegistrationTags(context.Background(), agent,
		[]string{ownTag.ID.String(), otherTag.ID.String()}, callerOrg, uuid.New())

	assert.Equal(t, []uuid.UUID{ownTag.ID}, repo.agentTags[agent.ID],
		"only the caller organization's tag is attached; the other organization's tag id is skipped")
}

func TestApplyRegistrationTags_AttachesNothingWhenEveryTagIDIsOfAnotherOrganization(t *testing.T) {
	callerOrg, otherOrg := uuid.New(), uuid.New()
	otherTag := registrationTag(otherOrg, "secret-project")
	repo := newRegistrationTagRepo(otherTag)
	service := &AgentService{tagRepo: repo}
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: callerOrg, Name: "caller-agent"}

	service.applyRegistrationTags(context.Background(), agent,
		[]string{otherTag.ID.String()}, callerOrg, uuid.New())

	_, called := repo.agentTags[agent.ID]
	assert.False(t, called, "no tag is attached when the only tag id is another organization's")
}

func TestApplyRegistrationTags_SkipsATagIDThatNamesNoTag(t *testing.T) {
	callerOrg := uuid.New()
	ownTag := registrationTag(callerOrg, "team")
	repo := newRegistrationTagRepo(ownTag)
	service := &AgentService{tagRepo: repo}
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: callerOrg, Name: "caller-agent"}

	service.applyRegistrationTags(context.Background(), agent,
		[]string{uuid.New().String(), ownTag.ID.String()}, callerOrg, uuid.New())

	assert.Equal(t, []uuid.UUID{ownTag.ID}, repo.agentTags[agent.ID])
}

func TestApplyRegistrationTags_TagNameFindsOnlyTheCallerOrganizationsTag(t *testing.T) {
	callerOrg, otherOrg := uuid.New(), uuid.New()
	ownTag := registrationTag(callerOrg, "team")
	otherTag := registrationTag(otherOrg, "team")
	repo := newRegistrationTagRepo(ownTag, otherTag)
	service := &AgentService{tagRepo: repo}
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: callerOrg, Name: "caller-agent"}

	service.applyRegistrationTags(context.Background(), agent, []string{"team"}, callerOrg, uuid.New())

	assert.Equal(t, []uuid.UUID{ownTag.ID}, repo.agentTags[agent.ID])
}

var tagByIDQuery = `FROM tags\s+WHERE id = \$1`

func tagRow(tag *domain.Tag) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "organization_id", "key", "value", "category", "description", "color", "created_at", "created_by",
	}).AddRow(tag.ID, tag.OrganizationID, tag.Key, tag.Value, tag.Category, tag.Description, tag.Color, tag.CreatedAt, tag.CreatedBy)
}

// The MCP server path runs against the real TagRepository: the only
// mcp_server_tags row written is the caller organization's tag.
func TestMCPServiceApplyRegistrationTags_InsertsOnlyTheCallerOrganizationsTag(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	callerOrg, otherOrg := uuid.New(), uuid.New()
	ownTag := registrationTag(callerOrg, "team")
	otherTag := registrationTag(otherOrg, "secret-project")
	unknownID := uuid.New()
	server := &domain.MCPServer{ID: uuid.New(), OrganizationID: callerOrg, Name: "caller-server"}
	service := &MCPService{tagRepo: repository.NewTagRepository(db)}

	mock.ExpectQuery(tagByIDQuery).WithArgs(unknownID).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(tagByIDQuery).WithArgs(ownTag.ID).WillReturnRows(tagRow(ownTag))
	mock.ExpectQuery(tagByIDQuery).WithArgs(otherTag.ID).WillReturnRows(tagRow(otherTag))
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO mcp_server_tags`).
		WithArgs(server.ID, ownTag.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	service.applyRegistrationTags(context.Background(), server,
		[]string{unknownID.String(), ownTag.ID.String(), otherTag.ID.String(), "not-a-uuid"}, callerOrg)

	require.NoError(t, mock.ExpectationsWereMet(),
		"each tag id is looked up, and only the caller organization's tag is inserted for the server")
}
