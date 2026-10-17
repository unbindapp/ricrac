package notification_service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type NotificationServiceSuite struct {
	services.ServiceTestSuite
	service *NotificationService
	actor   *auth.Actor
}

func (suite *NotificationServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.service = NewNotificationService(suite.MockRepo, mocks_services_resolve.NewResolveServiceMock(suite.T()))
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil, nil)
	suite.ExpectTx()
}

func TestNotificationService(t *testing.T) {
	suite.Run(t, new(NotificationServiceSuite))
}

func (suite *NotificationServiceSuite) mark(input MarkInput) string {
	_, err := suite.service.Mark(suite.Ctx, suite.actor, &input)
	return errdefs.As(err).Message
}

func (suite *NotificationServiceSuite) TestMarkingChecksItsArguments() {
	id := uuid.NewString()
	suite.Equal("Pass id or ids (at most 250)", suite.mark(MarkInput{Action: "read"}))
	suite.Equal(`No notification "x"`, suite.mark(MarkInput{IDs: []string{"x"}, Action: "read"}))
	suite.Equal("Pass until to snooze, e.g. PT3H, P1D or an ISO time", suite.mark(MarkInput{ID: &id, Action: "snooze"}))
	suite.Equal("until must be in the future", suite.mark(MarkInput{ID: &id, Action: "snooze", Until: new("2020-01-01T00:00:00Z")}))
	suite.Equal("until is only for snooze", suite.mark(MarkInput{ID: &id, Action: "read", Until: new("PT1H")}))
	suite.Equal(`action "open" isn't one of read, unread, archive, unarchive, snooze, unsnooze`, suite.mark(MarkInput{ID: &id, Action: "open"}))
}

func (suite *NotificationServiceSuite) TestOnlyYourNotificationsAreMarked() {
	mine, theirs := uuid.New(), uuid.New()
	suite.MockNotificationRepo.EXPECT().CountOwn(mock.Anything, mock.Anything, []uuid.UUID{mine, theirs}, suite.actor.User, suite.actor.Workspace).Return(1, nil)
	suite.Equal("No such notification in your inbox", suite.mark(MarkInput{ID: new(mine.String()), IDs: []string{theirs.String(), mine.String()}, Action: "read"}))
}
