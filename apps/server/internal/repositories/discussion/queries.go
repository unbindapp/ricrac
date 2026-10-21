package discussion_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/comment"
	"ricrac/ent/discussion"
	"ricrac/ent/discussionsubscriber"
	"ricrac/ent/discussionthread"
	"ricrac/ent/predicate"
	"ricrac/ent/user"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

// Load is the discussions with what their answer shows, in the order of ids.
func (self *DiscussionRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Discussion, error) {
	discussions, err := self.base.Client(tx).Discussion.Query().Where(discussion.IDIn(ids...)).
		WithWorkspace().WithTeam().WithCreator().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Discussion, len(discussions))
	for _, d := range discussions {
		byID[d.ID] = d
	}
	ordered := make([]*ent.Discussion, 0, len(ids))
	for _, id := range ids {
		if d, ok := byID[id]; ok {
			ordered = append(ordered, d)
		}
	}
	return ordered, nil
}

// Counts is how many threads, open threads and comments of their own each discussion has.
func (self *DiscussionRepository) Counts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]models.DiscussionCounts, error) {
	counts := make(map[uuid.UUID]models.DiscussionCounts, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}
	rows, err := self.base.Client(tx).QueryContext(ctx, `
		SELECT d.id,
			(SELECT count(*) FROM discussion_threads t WHERE t.discussion_id = d.id),
			(SELECT count(*) FROM discussion_threads t WHERE t.discussion_id = d.id AND t.resolved_at IS NULL),
			(SELECT count(*) FROM comments c WHERE c.discussion_id = d.id)
		FROM discussions d WHERE d.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var c models.DiscussionCounts
		if err := rows.Scan(&id, &c.Threads, &c.OpenThreads, &c.Comments); err != nil {
			return nil, err
		}
		counts[id] = c
	}
	return counts, rows.Err()
}

type ListFilter struct {
	Workspace uuid.UUID
	// The teams the caller sees: discussions of other teams are left out.
	Teams   []uuid.UUID
	Team    *uuid.UUID
	Creator *uuid.UUID
	Tag     *string
	// open, closed or all.
	Status         string
	IncludeTrashed bool
	// Only those in the trash, the most recently trashed first.
	OnlyTrashed bool
	// Words to find in the title or body.
	Query  *string
	Limit  int64
	Offset int64
}

// List is the discussions that match, pinned ones first, then by their latest activity (unless
// only trashed ones are asked for).
func (self *DiscussionRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Discussion.Query().Where(discussion.WorkspaceID(filter.Workspace), func(s *entsql.Selector) {
		s.Where(repository.Raw(s.C(discussion.FieldTeamID)+" IS NULL OR "+s.C(discussion.FieldTeamID)+" = ANY(?)", filter.Teams))
	})
	if filter.Team != nil {
		query = query.Where(discussion.TeamID(*filter.Team))
	}
	if filter.Creator != nil {
		query = query.Where(discussion.CreatorID(*filter.Creator))
	}
	if filter.Tag != nil {
		query = query.Where(func(s *entsql.Selector) {
			s.Where(repository.Raw("? = ANY("+s.C(discussion.FieldTags)+")", *filter.Tag))
		})
	}
	switch filter.Status {
	case "open":
		query = query.Where(discussion.ClosedAtIsNil())
	case "closed":
		query = query.Where(discussion.ClosedAtNotNil())
	}
	switch {
	case filter.OnlyTrashed:
		query = query.Where(discussion.TrashedAtNotNil()).Order(ent.Desc(discussion.FieldTrashedAt))
	case !filter.IncludeTrashed:
		query = query.Where(discussion.TrashedAtIsNil())
	}
	if filter.Query != nil {
		words := *filter.Query
		query = query.Where(func(s *entsql.Selector) {
			s.Where(repository.Raw("("+s.C("search")+" @@ websearch_to_tsquery('english', ?) OR "+s.C(discussion.FieldTitle)+" ILIKE ?)",
				words, "%"+utils.EscapeLike(words)+"%"))
		})
	}
	return query.Order(func(s *entsql.Selector) {
		s.OrderExpr(repository.RawExpr(s.C(discussion.FieldPinnedAt) + " IS NULL"))
	}, ent.Desc(discussion.FieldLastActivityAt), ent.Asc(discussion.FieldID)).
		Limit(int(filter.Limit)).Offset(int(filter.Offset)).IDs(ctx)
}

// Owner is the team a discussion belongs to (nil for the whole workspace) and whether it is in
// the trash; found is false when the workspace has no such discussion.
func (self *DiscussionRepository) Owner(ctx context.Context, workspace, id uuid.UUID) (team *uuid.UUID, trashed, found bool, err error) {
	row, err := self.base.DB.Discussion.Query().Where(discussion.ID(id), discussion.WorkspaceID(workspace)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, false, false, nil
	}
	if err != nil {
		return nil, false, false, err
	}
	return row.TeamID, row.TrashedAt != nil, true, nil
}

// GetForUpdate locks the discussion until the transaction ends.
func (self *DiscussionRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Discussion, error) {
	return self.base.Client(tx).Discussion.Query().Where(discussion.ID(id)).ForUpdate().Only(ctx)
}

// Trashed is the discussion in the trash most recently trashed whose id (asID), slug id or title
// (in any case) matches.
func (self *DiscussionRepository) Trashed(ctx context.Context, workspace uuid.UUID, asID *uuid.UUID, slug, title string) (uuid.UUID, bool, error) {
	matches := []predicate.Discussion{discussion.SlugID(slug), func(s *entsql.Selector) {
		s.Where(repository.Raw("lower("+s.C(discussion.FieldTitle)+") = lower(?)", title))
	}}
	if asID != nil {
		matches = append(matches, discussion.ID(*asID))
	}
	ids, err := self.base.DB.Discussion.Query().
		Where(discussion.WorkspaceID(workspace), discussion.TrashedAtNotNil(), discussion.Or(matches...)).
		Order(ent.Desc(discussion.FieldTrashedAt)).Limit(1).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}

func (self *DiscussionRepository) Subscribers(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.User, error) {
	return self.base.Client(tx).User.Query().
		Where(user.HasSubscribedDiscussionsWith(discussion.ID(id))).Order(ent.Asc(user.FieldName)).All(ctx)
}

func (self *DiscussionRepository) SubscriberIDs(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]uuid.UUID, error) {
	rows, err := self.base.Client(tx).DiscussionSubscriber.Query().Where(discussionsubscriber.DiscussionID(id)).All(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		users[i] = r.UserID
	}
	return users, nil
}

func withThreadAnswer(q *ent.DiscussionThreadQuery) *ent.DiscussionThreadQuery {
	return q.WithDiscussion(func(q *ent.DiscussionQuery) { q.WithWorkspace() }).WithCreator().WithResolvingUser().
		WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() })
}

// Threads is the discussion's threads with what their answer shows, oldest first.
func (self *DiscussionRepository) Threads(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.DiscussionThread, error) {
	return withThreadAnswer(self.base.Client(tx).DiscussionThread.Query()).Where(discussionthread.DiscussionID(id)).
		Order(ent.Asc(discussionthread.FieldCreatedAt), ent.Asc(discussionthread.FieldID)).All(ctx)
}

// Thread is the thread with what its answer shows.
func (self *DiscussionRepository) Thread(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.DiscussionThread, error) {
	return withThreadAnswer(self.base.Client(tx).DiscussionThread.Query()).Where(discussionthread.ID(id)).Only(ctx)
}

// FindThread is a thread of the workspace, nil when there's none.
func (self *DiscussionRepository) FindThread(ctx context.Context, workspace, id uuid.UUID) (*ent.DiscussionThread, error) {
	found, err := self.base.DB.DiscussionThread.Query().
		Where(discussionthread.ID(id), discussionthread.WorkspaceID(workspace)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// ThreadCommentCounts is how many comments each thread has.
func (self *DiscussionRepository) ThreadCommentCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		ID    uuid.UUID `json:"discussion_thread_id"`
		Count int       `json:"count"`
	}
	err := self.base.Client(tx).Comment.Query().Where(comment.DiscussionThreadIDIn(ids...)).
		GroupBy(comment.FieldDiscussionThreadID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		counts[r.ID] = r.Count
	}
	return counts, nil
}

// Get is the discussion with its workspace.
func (self *DiscussionRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Discussion, error) {
	return self.base.Client(tx).Discussion.Query().Where(discussion.ID(id)).WithWorkspace().Only(ctx)
}

// Tags is the tags on the discussions the caller sees outside the trash, the most used first.
func (self *DiscussionRepository) Tags(ctx context.Context, workspace uuid.UUID, teams []uuid.UUID, limit, offset int64) ([]models.DiscussionTag, error) {
	rows, err := self.base.DB.QueryContext(ctx, `
		SELECT tag, count(*) FROM discussions d, unnest(d.tags) AS tag
		WHERE d.workspace_id = $1 AND d.trashed_at IS NULL AND (d.team_id IS NULL OR d.team_id = ANY($2))
		GROUP BY tag ORDER BY count(*) DESC, tag LIMIT $3 OFFSET $4`, workspace, teams, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []models.DiscussionTag{}
	for rows.Next() {
		var tag models.DiscussionTag
		if err := rows.Scan(&tag.Name, &tag.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}
