// Package services holds what the services' unit tests share: the repositories as mocks.
package services

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	repository "ricrac/internal/repositories"
	mocks_repositories "ricrac/mocks/repositories"
	mocks_repository_access "ricrac/mocks/repository/access"
	mocks_repository_account "ricrac/mocks/repository/account"
	mocks_repository_attachment "ricrac/mocks/repository/attachment"
	mocks_repository_comment "ricrac/mocks/repository/comment"
	mocks_repository_customer "ricrac/mocks/repository/customer"
	mocks_repository_cycle "ricrac/mocks/repository/cycle"
	mocks_repository_document "ricrac/mocks/repository/document"
	mocks_repository_email "ricrac/mocks/repository/email"
	mocks_repository_events "ricrac/mocks/repository/events"
	mocks_repository_favorite "ricrac/mocks/repository/favorite"
	mocks_repository_issue "ricrac/mocks/repository/issue"
	mocks_repository_label "ricrac/mocks/repository/label"
	mocks_repository_notification "ricrac/mocks/repository/notification"
	mocks_repository_oauth "ricrac/mocks/repository/oauth"
	mocks_repository_release "ricrac/mocks/repository/release"
	mocks_repository_resolve "ricrac/mocks/repository/resolve"
	mocks_repository_search "ricrac/mocks/repository/search"
	mocks_repository_seed "ricrac/mocks/repository/seed"
	mocks_repository_state "ricrac/mocks/repository/state"
	mocks_repository_team "ricrac/mocks/repository/team"
	mocks_repository_template "ricrac/mocks/repository/template"
	mocks_repository_tx "ricrac/mocks/repository/tx"
	mocks_repository_webhook "ricrac/mocks/repository/webhook"
	mocks_repository_workspace "ricrac/mocks/repository/workspace"
)

type ServiceTestSuite struct {
	suite.Suite
	Ctx context.Context

	MockRepo             *mocks_repositories.RepositoriesMock
	MockAccessRepo       *mocks_repository_access.AccessRepositoryMock
	MockResolveRepo      *mocks_repository_resolve.ResolveRepositoryMock
	MockEventsRepo       *mocks_repository_events.EventsRepositoryMock
	MockLabelRepo        *mocks_repository_label.LabelRepositoryMock
	MockSeedRepo         *mocks_repository_seed.SeedRepositoryMock
	MockAttachmentRepo   *mocks_repository_attachment.AttachmentRepositoryMock
	MockCommentRepo      *mocks_repository_comment.CommentRepositoryMock
	MockIssueRepo        *mocks_repository_issue.IssueRepositoryMock
	MockNotificationRepo *mocks_repository_notification.NotificationRepositoryMock
	MockSearchRepo       *mocks_repository_search.SearchRepositoryMock
	MockAccountRepo      *mocks_repository_account.AccountRepositoryMock
	MockOAuthRepo        *mocks_repository_oauth.OAuthRepositoryMock
	MockWorkspaceRepo    *mocks_repository_workspace.WorkspaceRepositoryMock
	MockTeamRepo         *mocks_repository_team.TeamRepositoryMock
	MockStateRepo        *mocks_repository_state.StateRepositoryMock
	MockEmailRepo        *mocks_repository_email.EmailRepositoryMock
	MockCycleRepo        *mocks_repository_cycle.CycleRepositoryMock
	MockDocumentRepo     *mocks_repository_document.DocumentRepositoryMock
	MockFavoriteRepo     *mocks_repository_favorite.FavoriteRepositoryMock
	MockTemplateRepo     *mocks_repository_template.TemplateRepositoryMock
	MockCustomerRepo     *mocks_repository_customer.CustomerRepositoryMock
	MockReleaseRepo      *mocks_repository_release.ReleaseRepositoryMock
	MockWebhookRepo      *mocks_repository_webhook.WebhookRepositoryMock
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.Ctx = context.Background()
	suite.MockRepo = mocks_repositories.NewRepositoriesMock(suite.T())
	suite.MockAccessRepo = mocks_repository_access.NewAccessRepositoryMock(suite.T())
	suite.MockResolveRepo = mocks_repository_resolve.NewResolveRepositoryMock(suite.T())
	suite.MockEventsRepo = mocks_repository_events.NewEventsRepositoryMock(suite.T())
	suite.MockLabelRepo = mocks_repository_label.NewLabelRepositoryMock(suite.T())
	suite.MockSeedRepo = mocks_repository_seed.NewSeedRepositoryMock(suite.T())
	suite.MockAttachmentRepo = mocks_repository_attachment.NewAttachmentRepositoryMock(suite.T())
	suite.MockCommentRepo = mocks_repository_comment.NewCommentRepositoryMock(suite.T())
	suite.MockIssueRepo = mocks_repository_issue.NewIssueRepositoryMock(suite.T())
	suite.MockNotificationRepo = mocks_repository_notification.NewNotificationRepositoryMock(suite.T())
	suite.MockSearchRepo = mocks_repository_search.NewSearchRepositoryMock(suite.T())
	suite.MockAccountRepo = mocks_repository_account.NewAccountRepositoryMock(suite.T())
	suite.MockOAuthRepo = mocks_repository_oauth.NewOAuthRepositoryMock(suite.T())
	suite.MockWorkspaceRepo = mocks_repository_workspace.NewWorkspaceRepositoryMock(suite.T())
	suite.MockTeamRepo = mocks_repository_team.NewTeamRepositoryMock(suite.T())
	suite.MockStateRepo = mocks_repository_state.NewStateRepositoryMock(suite.T())
	suite.MockEmailRepo = mocks_repository_email.NewEmailRepositoryMock(suite.T())
	suite.MockCycleRepo = mocks_repository_cycle.NewCycleRepositoryMock(suite.T())
	suite.MockDocumentRepo = mocks_repository_document.NewDocumentRepositoryMock(suite.T())
	suite.MockFavoriteRepo = mocks_repository_favorite.NewFavoriteRepositoryMock(suite.T())
	suite.MockTemplateRepo = mocks_repository_template.NewTemplateRepositoryMock(suite.T())
	suite.MockCustomerRepo = mocks_repository_customer.NewCustomerRepositoryMock(suite.T())
	suite.MockReleaseRepo = mocks_repository_release.NewReleaseRepositoryMock(suite.T())
	suite.MockWebhookRepo = mocks_repository_webhook.NewWebhookRepositoryMock(suite.T())

	suite.MockRepo.EXPECT().Access().Return(suite.MockAccessRepo).Maybe()
	suite.MockRepo.EXPECT().Resolve().Return(suite.MockResolveRepo).Maybe()
	suite.MockRepo.EXPECT().Events().Return(suite.MockEventsRepo).Maybe()
	suite.MockRepo.EXPECT().Label().Return(suite.MockLabelRepo).Maybe()
	suite.MockRepo.EXPECT().Seed().Return(suite.MockSeedRepo).Maybe()
	suite.MockRepo.EXPECT().Attachment().Return(suite.MockAttachmentRepo).Maybe()
	suite.MockRepo.EXPECT().Comment().Return(suite.MockCommentRepo).Maybe()
	suite.MockRepo.EXPECT().Issue().Return(suite.MockIssueRepo).Maybe()
	suite.MockRepo.EXPECT().Notification().Return(suite.MockNotificationRepo).Maybe()
	suite.MockRepo.EXPECT().Search().Return(suite.MockSearchRepo).Maybe()
	suite.MockRepo.EXPECT().Account().Return(suite.MockAccountRepo).Maybe()
	suite.MockRepo.EXPECT().OAuth().Return(suite.MockOAuthRepo).Maybe()
	suite.MockRepo.EXPECT().Workspace().Return(suite.MockWorkspaceRepo).Maybe()
	suite.MockRepo.EXPECT().Team().Return(suite.MockTeamRepo).Maybe()
	suite.MockRepo.EXPECT().State().Return(suite.MockStateRepo).Maybe()
	suite.MockRepo.EXPECT().Email().Return(suite.MockEmailRepo).Maybe()
	suite.MockRepo.EXPECT().Cycle().Return(suite.MockCycleRepo).Maybe()
	suite.MockRepo.EXPECT().Document().Return(suite.MockDocumentRepo).Maybe()
	suite.MockRepo.EXPECT().Favorite().Return(suite.MockFavoriteRepo).Maybe()
	suite.MockRepo.EXPECT().Template().Return(suite.MockTemplateRepo).Maybe()
	suite.MockRepo.EXPECT().Customer().Return(suite.MockCustomerRepo).Maybe()
	suite.MockRepo.EXPECT().Release().Return(suite.MockReleaseRepo).Maybe()
	suite.MockRepo.EXPECT().Webhook().Return(suite.MockWebhookRepo).Maybe()
}

// ExpectTx runs WithTx bodies directly; repository mocks match the tx argument with mock.Anything.
func (suite *ServiceTestSuite) ExpectTx() {
	suite.MockRepo.EXPECT().WithTx(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(repository.TxInterface) error) error {
		return fn(mocks_repository_tx.NewTxMock(suite.T()))
	}).Maybe()
}
