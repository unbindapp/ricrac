// Package services holds what the services' unit tests share: the repositories as mocks.
package services

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	repository "ricrac/internal/repositories"
	mocks_repositories "ricrac/mocks/repositories"
	mocks_repository_access "ricrac/mocks/repository/access"
	mocks_repository_attachment "ricrac/mocks/repository/attachment"
	mocks_repository_comment "ricrac/mocks/repository/comment"
	mocks_repository_customer "ricrac/mocks/repository/customer"
	mocks_repository_events "ricrac/mocks/repository/events"
	mocks_repository_issue "ricrac/mocks/repository/issue"
	mocks_repository_label "ricrac/mocks/repository/label"
	mocks_repository_release "ricrac/mocks/repository/release"
	mocks_repository_resolve "ricrac/mocks/repository/resolve"
	mocks_repository_seed "ricrac/mocks/repository/seed"
	mocks_repository_tx "ricrac/mocks/repository/tx"
	mocks_repository_webhook "ricrac/mocks/repository/webhook"
)

type ServiceTestSuite struct {
	suite.Suite
	Ctx context.Context

	MockRepo           *mocks_repositories.RepositoriesMock
	MockAccessRepo     *mocks_repository_access.AccessRepositoryMock
	MockResolveRepo    *mocks_repository_resolve.ResolveRepositoryMock
	MockEventsRepo     *mocks_repository_events.EventsRepositoryMock
	MockLabelRepo      *mocks_repository_label.LabelRepositoryMock
	MockSeedRepo       *mocks_repository_seed.SeedRepositoryMock
	MockAttachmentRepo *mocks_repository_attachment.AttachmentRepositoryMock
	MockCommentRepo    *mocks_repository_comment.CommentRepositoryMock
	MockIssueRepo      *mocks_repository_issue.IssueRepositoryMock
	MockCustomerRepo   *mocks_repository_customer.CustomerRepositoryMock
	MockReleaseRepo    *mocks_repository_release.ReleaseRepositoryMock
	MockWebhookRepo    *mocks_repository_webhook.WebhookRepositoryMock
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
