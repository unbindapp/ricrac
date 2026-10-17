package comment_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	comment_repo "ricrac/internal/repositories/comment"
)

// Target is what a comment is on: the column of comments holding its id (see comment_repo.Targets)
// and the id.
type Target struct {
	Column string
	ID     uuid.UUID
}

type targetInput struct {
	IssueID        *string `json:"issueId" doc:"Issue identifier (ENG-123), id or link."`
	ProjectID      *string `json:"projectId" doc:"Project name, slug or id."`
	InitiativeID   *string `json:"initiativeId" doc:"Initiative name, slug or id."`
	DocumentID     *string `json:"documentId" doc:"Document title, slug or id."`
	MilestoneID    *string `json:"milestoneId" doc:"Project milestone id."`
	StatusUpdateID *string `json:"statusUpdateId" doc:"Status update id."`
}

func (t targetInput) given() []*string {
	return []*string{t.IssueID, t.ProjectID, t.InitiativeID, t.DocumentID, t.MilestoneID, t.StatusUpdateID}
}

// TargetOf is what a comment is on.
func TargetOf(c *ent.Comment) (Target, error) {
	for i, id := range []*uuid.UUID{c.IssueID, c.ProjectID, c.InitiativeID, c.DocumentID, c.MilestoneID, c.StatusUpdateID} {
		if id != nil {
			return Target{comment_repo.Targets[i], *id}, nil
		}
	}
	return Target{}, errdefs.NotFound("That comment is on nothing")
}

// namedTarget is the one thing named among the arguments, checked visible, or nil when none is
// named.
func (self *CommentService) namedTarget(ctx context.Context, actor *auth.Actor, input targetInput) (*Target, error) {
	var column, value string
	count := 0
	for i, v := range input.given() {
		if v != nil {
			column, value = comment_repo.Targets[i], *v
			count++
		}
	}
	if count == 0 {
		return nil, nil
	}
	if count > 1 {
		return nil, errdefs.BadRequest("Pass only one of issueId, projectId, initiativeId, documentId, milestoneId and statusUpdateId")
	}
	var id uuid.UUID
	var err error
	switch column {
	case "issue_id":
		id, err = self.resolve.Issue(ctx, actor, value)
	case "project_id":
		id, err = self.resolve.Project(ctx, actor, value)
	case "initiative_id":
		id, err = self.resolve.Initiative(ctx, actor, value)
	case "document_id":
		id, err = self.resolve.Document(ctx, actor, value)
	case "milestone_id":
		id, err = self.resolve.Milestone(ctx, actor, nil, value)
	default:
		if id, err = self.resolve.ByID(ctx, actor, "status_updates", "status update", value); err == nil {
			err = self.checkVisible(ctx, actor, Target{"status_update_id", id})
		}
	}
	if err != nil {
		return nil, err
	}
	return &Target{column, id}, nil
}

func (self *CommentService) checkVisible(ctx context.Context, actor *auth.Actor, target Target) error {
	id := target.ID.String()
	var visible bool
	var err error
	switch target.Column {
	case "issue_id":
		visible, err = self.resolve.CanSeeIssue(ctx, actor, target.ID)
	case "project_id":
		_, resolveErr := self.resolve.Project(ctx, actor, id)
		visible = resolveErr == nil
	case "initiative_id":
		_, resolveErr := self.resolve.Initiative(ctx, actor, id)
		visible = resolveErr == nil
	case "document_id":
		visible, err = self.resolve.CanSeeDocument(ctx, actor, target.ID)
	case "milestone_id":
		_, resolveErr := self.resolve.Milestone(ctx, actor, nil, id)
		visible = resolveErr == nil
	default:
		var update *ent.StatusUpdate
		update, err = self.repo.Comment().StatusUpdate(ctx, nil, target.ID)
		switch {
		case update == nil:
		case update.ProjectID != nil:
			_, resolveErr := self.resolve.Project(ctx, actor, update.ProjectID.String())
			visible = resolveErr == nil
		case update.InitiativeID != nil:
			_, resolveErr := self.resolve.Initiative(ctx, actor, update.InitiativeID.String())
			visible = resolveErr == nil
		}
	}
	if err != nil {
		return err
	}
	if !visible {
		return errdefs.NotFound("Not found, or you can't see it")
	}
	return nil
}

// Load is a comment of the workspace by id, on something the caller can see.
func (self *CommentService) Load(ctx context.Context, actor *auth.Actor, value string) (*ent.Comment, error) {
	notFound := errdefs.NotFound("No comment %q", strings.TrimSpace(value))
	id, ok := utils.AsUUID(value)
	if !ok {
		return nil, notFound
	}
	found, err := self.repo.Comment().Find(ctx, actor.Workspace, id)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, notFound
	}
	target, err := TargetOf(found)
	if err != nil {
		return nil, err
	}
	if self.checkVisible(ctx, actor, target) != nil {
		return nil, notFound
	}
	return found, nil
}

// detail is the comment as saving it answers, read in tx.
func (self *CommentService) detail(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.CommentDetail, error) {
	found, err := self.repo.Comment().Load(ctx, tx, id)
	if err != nil {
		return models.CommentDetail{}, err
	}
	if found == nil {
		return models.CommentDetail{}, errdefs.NotFound("No such comment")
	}
	return models.CommentDetailOf(found, self.cfg.PublicURL), nil
}

// teamOf is the team of the issue a comment is on, for webhooks.
func (self *CommentService) teamOf(ctx context.Context, tx repository.TxInterface, target Target) (*uuid.UUID, error) {
	if target.Column != "issue_id" {
		return nil, nil
	}
	return self.repo.Comment().IssueTeam(ctx, tx, target.ID)
}
