package comment_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type CommentServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	events  *mocks_services_events.EventsServiceMock
	service *CommentService
	actor   *auth.Actor
}

func (suite *CommentServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.events = mocks_services_events.NewEventsServiceMock(suite.T())
	suite.service = NewCommentService(config.Config{}, suite.MockRepo, suite.resolve, suite.events)
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
	suite.ExpectTx()
}

func TestCommentService(t *testing.T) {
	suite.Run(t, new(CommentServiceSuite))
}

// projectComment is a comment on a project the actor can see.
func (suite *CommentServiceSuite) projectComment(author uuid.UUID, parent *uuid.UUID) *ent.Comment {
	project := uuid.New()
	comment := &ent.Comment{ID: uuid.New(), WorkspaceID: suite.actor.Workspace, ProjectID: &project, UserID: &author, ParentID: parent}
	suite.MockCommentRepo.EXPECT().Find(mock.Anything, suite.actor.Workspace, comment.ID).Return(comment, nil)
	suite.resolve.EXPECT().Project(mock.Anything, suite.actor, project.String()).Return(project, nil)
	return comment
}

func (suite *CommentServiceSuite) TestCommentsAreOnOneThing() {
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{
		Body: new("Hi"), targetInput: targetInput{IssueID: new("ENG-1"), ProjectID: new("Launch")},
	})
	suite.Equal("Pass only one of issueId, projectId, initiativeId, documentId, milestoneId and statusUpdateId", errdefs.As(err).Message)
	_, err = suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Body: new("Hi")})
	suite.Equal("Pass what to comment on (issueId, projectId, initiativeId, documentId, milestoneId or statusUpdateId) or parentId", errdefs.As(err).Message)
}

func (suite *CommentServiceSuite) TestEditsOnlyChangeTheBody() {
	suite.resolve.EXPECT().Issue(mock.Anything, suite.actor, "ENG-1").Return(uuid.New(), nil)
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{ID: new(uuid.NewString()), Body: new("Hi"), targetInput: targetInput{IssueID: new("ENG-1")}})
	suite.Equal("Only the body of a comment can be changed", errdefs.As(err).Message)
}

func (suite *CommentServiceSuite) TestOnlyTheAuthorEdits() {
	comment := suite.projectComment(uuid.New(), nil)
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{ID: new(comment.ID.String()), Body: new("Hi")})
	suite.Equal("Only its author can edit a comment", errdefs.As(err).Message)
}

func (suite *CommentServiceSuite) TestOnlyTheAuthorOrAdminsDelete() {
	comment := suite.projectComment(uuid.New(), nil)
	_, err := suite.service.Delete(suite.Ctx, suite.actor, &IDInput{ID: comment.ID.String()})
	suite.Equal(403, errdefs.As(err).Status)
}

func (suite *CommentServiceSuite) TestRepliesArentResolved() {
	comment := suite.projectComment(suite.actor.User, new(uuid.New()))
	_, err := suite.service.Resolve(suite.Ctx, suite.actor, &IDInput{ID: comment.ID.String()})
	suite.Equal("Only a thread is resolved; pass the thread's first comment (the parentId)", errdefs.As(err).Message)
}

func (suite *CommentServiceSuite) TestHiddenCommentsArentFound() {
	project := uuid.New()
	comment := &ent.Comment{ID: uuid.New(), ProjectID: &project}
	suite.MockCommentRepo.EXPECT().Find(mock.Anything, suite.actor.Workspace, comment.ID).Return(comment, nil)
	suite.resolve.EXPECT().Project(mock.Anything, suite.actor, project.String()).Return(uuid.Nil, errdefs.NotFound("No project"))
	_, err := suite.service.Load(suite.Ctx, suite.actor, " "+comment.ID.String()+" ")
	suite.Equal(`No comment "`+comment.ID.String()+`"`, errdefs.As(err).Message)
}

func (suite *CommentServiceSuite) TestReactionsAreOnOneThing() {
	_, err := suite.service.AddReaction(suite.Ctx, suite.actor, &ReactionInput{Emoji: "+1"})
	suite.Equal("Pass exactly one of issueId, commentId and statusUpdateId", errdefs.As(err).Message)
	_, err = suite.service.RemoveReaction(suite.Ctx, suite.actor, &RemoveReactionInput{ID: new("x"), Emoji: new("+1")})
	suite.Equal("Pass the reaction's id, or emoji and what it is on", errdefs.As(err).Message)
}

func TestEmojiAreShortcodesOrUnicode(t *testing.T) {
	for value, want := range map[string]string{":tada:": "tada", "+1": "+1", "👍": "👍", " :Thumbs_Up: ": "thumbs_up"} {
		if got, err := normalizeEmoji(value); err != nil || got != want {
			t.Errorf("%s: got %s, %v", value, got, err)
		}
	}
	for _, value := range []string{"not an emoji", "::", ""} {
		if _, err := normalizeEmoji(value); err == nil {
			t.Errorf("%s should fail", value)
		}
	}
}
