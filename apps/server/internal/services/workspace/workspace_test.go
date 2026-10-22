package workspace_service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_email "ricrac/mocks/services/email"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type WorkspaceServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	service *WorkspaceService
	admin   *auth.Actor
	user    uuid.UUID
}

func (suite *WorkspaceServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.service = NewWorkspaceService(config.Config{}, suite.MockRepo, suite.resolve,
		mocks_services_events.NewEventsServiceMock(suite.T()), mocks_services_email.NewEmailServiceMock(suite.T()))
	suite.admin = auth.NewActor(uuid.New(), uuid.New(), auth.Admin, auth.EveryAccess, nil, nil)
	suite.user = uuid.New()
	suite.ExpectTx()
}

func TestWorkspaceService(t *testing.T) {
	suite.Run(t, new(WorkspaceServiceSuite))
}

func (suite *WorkspaceServiceSuite) TestOnlyOwnersTouchOwnership() {
	suite.MockWorkspaceRepo.EXPECT().RoleForUpdate(mock.Anything, mock.Anything, suite.admin.Workspace, suite.user).Return("member", nil)
	err := suite.service.changeRole(suite.Ctx, nil, suite.admin, suite.user, "owner")
	suite.Equal("Only owners can make someone an owner or take it away", errdefs.As(err).Message)
}

func (suite *WorkspaceServiceSuite) TestTheLastOwnerStaysOne() {
	owner := auth.NewActor(suite.user, suite.admin.Workspace, auth.Owner, auth.EveryAccess, nil, nil)
	suite.MockWorkspaceRepo.EXPECT().RoleForUpdate(mock.Anything, mock.Anything, owner.Workspace, suite.user).Return("owner", nil)
	suite.MockWorkspaceRepo.EXPECT().ActiveOwners(mock.Anything, mock.Anything, owner.Workspace).Return(1, nil)
	err := suite.service.changeRole(suite.Ctx, nil, owner, suite.user, "admin")
	suite.Equal("The workspace needs an owner; make someone else owner first", errdefs.As(err).Message)
}

func (suite *WorkspaceServiceSuite) TestPeopleOnlyChangeTheirOwnProfile() {
	suite.resolve.EXPECT().User(mock.Anything, suite.admin, "bob").Return(suite.user, nil)
	_, err := suite.service.UpdateUser(suite.Ctx, suite.admin, &UpdateUserInput{ID: "bob", Name: new("Robert")})
	suite.Equal(403, errdefs.As(err).Status)
}

func (suite *WorkspaceServiceSuite) TestInvitationsCheckEmailAndRole() {
	member := auth.NewActor(uuid.New(), suite.admin.Workspace, auth.Member, auth.EveryAccess, nil, nil)
	_, err := suite.service.Invite(suite.Ctx, member, &InviteInput{Email: "not an email"})
	suite.Equal(`"not an email" isn't an email address`, errdefs.As(err).Message)
	_, err = suite.service.Invite(suite.Ctx, member, &InviteInput{Email: "new@acme.test", Role: new("admin")})
	suite.Equal("Only admins can invite admins", errdefs.As(err).Message)
	_, err = suite.service.Invite(suite.Ctx, member, &InviteInput{Email: "new@acme.test", Role: new("owner")})
	suite.Equal(`role is member, admin or guest, not "owner"`, errdefs.As(err).Message)
}
