package account_service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_access "ricrac/mocks/services/access"
	mocks_services_seed "ricrac/mocks/services/seed"
)

type AccountServiceSuite struct {
	services.ServiceTestSuite
	access  *mocks_services_access.AccessServiceMock
	seed    *mocks_services_seed.SeedServiceMock
	service *AccountService
	user    uuid.UUID
}

func (suite *AccountServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.access = mocks_services_access.NewAccessServiceMock(suite.T())
	suite.seed = mocks_services_seed.NewSeedServiceMock(suite.T())
	suite.service = NewAccountService(config.Config{PublicURL: "https://ricrac.example"}, suite.MockRepo, suite.access, suite.seed)
	suite.user = uuid.New()
	suite.ExpectTx()
}

func TestAccountService(t *testing.T) {
	suite.Run(t, new(AccountServiceSuite))
}

func message(err error) string {
	return errdefs.As(err).Message
}

func (suite *AccountServiceSuite) TestScopesAreOrderedAndImplyRead() {
	checked, err := CheckScopes([]string{"admin", "write"})
	suite.NoError(err)
	suite.Equal([]string{"read", "write", "admin"}, checked)
	all, _ := CheckScopes(nil)
	suite.Equal(auth.AllScopes, all)
	_, err = CheckScopes([]string{})
	suite.Equal("Pick at least one scope", message(err))
	_, err = CheckScopes([]string{"everything"})
	suite.Equal(`Unknown scope "everything"`, message(err))
}

func (suite *AccountServiceSuite) TestGrantsOnlyReachTheUsersWorkspaces() {
	mine, theirs := uuid.New(), uuid.New()
	suite.NoError(suite.service.CheckWorkspaces(suite.Ctx, suite.user, nil), "nil is every workspace")
	suite.Equal("Pick at least one workspace.", message(suite.service.CheckWorkspaces(suite.Ctx, suite.user, []uuid.UUID{})))
	suite.MockAccountRepo.EXPECT().ActiveIn(mock.Anything, suite.user, []uuid.UUID{mine, theirs}).Return([]uuid.UUID{mine}, nil)
	err := suite.service.CheckWorkspaces(suite.Ctx, suite.user, []uuid.UUID{mine, theirs})
	suite.Equal("You aren't a member of one of those workspaces.", message(err))
}

func (suite *AccountServiceSuite) TestTheOnlyOwnerCantLeave() {
	workspace := uuid.New()
	suite.MockAccountRepo.EXPECT().LockOwners(mock.Anything, mock.Anything, workspace).Return([]uuid.UUID{suite.user}, nil)
	err := suite.service.LeaveWorkspace(suite.Ctx, suite.user, workspace)
	suite.Equal("You're the only owner: make someone else an owner first, or delete the workspace", message(err))
}

func (suite *AccountServiceSuite) TestDisplayNamesAreHandles() {
	err := suite.service.UpdateMe(suite.Ctx, suite.user, &UpdateMe{DisplayName: new("ann smith")})
	suite.Equal("Display names are 1 to 40 letters, digits, dots, dashes or underscores", message(err))
	suite.MockAccountRepo.EXPECT().SetDisplayName(mock.Anything, suite.user, "ann.s").Return(nil)
	suite.NoError(suite.service.UpdateMe(suite.Ctx, suite.user, &UpdateMe{DisplayName: new(" ann.s ")}))
}

func (suite *AccountServiceSuite) TestOnlyTheCreatorOrAnAdminChangesAnApp() {
	workspace, creator := uuid.New(), uuid.New()
	suite.MockAccountRepo.EXPECT().OAuthApp(mock.Anything, "app", workspace).Return(&ent.OAuthClient{ID: "app", OwnerID: &creator}, nil)
	member := auth.NewActor(suite.user, workspace, auth.Member, auth.EveryAccess, nil, nil)
	suite.Equal("Only the app's creator or an admin can change it", message(suite.service.CheckAppOwner(suite.Ctx, member, "app")))
	admin := auth.NewActor(suite.user, workspace, auth.Admin, auth.EveryAccess, nil, nil)
	suite.NoError(suite.service.CheckAppOwner(suite.Ctx, admin, "app"))
	creatorActor := auth.NewActor(creator, workspace, auth.Member, auth.EveryAccess, nil, nil)
	suite.NoError(suite.service.CheckAppOwner(suite.Ctx, creatorActor, "app"))
}

func (suite *AccountServiceSuite) TestAppsNeedAllowedCallbacks() {
	err := checkApp(&SaveOAuthApp{Name: "App", RedirectURIs: []string{" http://agent.example/cb "}})
	suite.Equal("Callback URL not allowed:  http://agent.example/cb  (https, http on localhost, or an app scheme)", message(err))
	err = checkApp(&SaveOAuthApp{Name: "App", RedirectURIs: []string{"https://agent.example/cb"}, HomepageURL: new("agent.example")})
	suite.Equal("The homepage is an http(s) URL", message(err))
	suite.NoError(checkApp(&SaveOAuthApp{Name: "App", RedirectURIs: []string{" http://localhost:9999/cb "}, HomepageURL: new("  ")}))
}
