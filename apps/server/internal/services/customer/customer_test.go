package customer_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	customer_repo "ricrac/internal/repositories/customer"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type CustomerServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	events  *mocks_services_events.EventsServiceMock
	service *CustomerService
	actor   *auth.Actor
}

func (suite *CustomerServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.events = mocks_services_events.NewEventsServiceMock(suite.T())
	suite.service = NewCustomerService(suite.MockRepo, suite.resolve, suite.events)
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
	suite.ExpectTx()
}

func TestCustomerService(t *testing.T) {
	suite.Run(t, new(CustomerServiceSuite))
}

func (suite *CustomerServiceSuite) TestDomainsAreHostsInLowercase() {
	for value, want := range map[string]string{"https://Globex.com/": "globex.com", " acme.io ": "acme.io", "http://a.b/c": "a.b"} {
		got, err := customerDomain(value)
		suite.NoError(err)
		suite.Equal(want, got)
	}
	for _, value := range []string{"localhost", ".com", "acme.", "ac me.com"} {
		_, err := customerDomain(value)
		suite.Equal(400, errdefs.As(err).Status, value)
	}
}

func (suite *CustomerServiceSuite) TestGuestsCantSeeCustomers() {
	guest := auth.NewActor(uuid.New(), suite.actor.Workspace, auth.Guest, auth.EveryAccess, nil, nil)
	_, err := suite.service.List(suite.Ctx, guest, &ListInput{})
	suite.Equal(403, errdefs.As(err).Status)
	_, err = suite.service.Get(suite.Ctx, guest, &IDInput{ID: "Globex"})
	suite.Equal(403, errdefs.As(err).Status)
}

func (suite *CustomerServiceSuite) TestUnknownOptionsListTheOthers() {
	suite.MockCustomerRepo.EXPECT().Named(mock.Anything, customer_repo.Tiers, suite.actor.Workspace).
		Return([]customer_repo.Option{{ID: uuid.New(), Name: "Growth"}, {ID: uuid.New(), Name: "Enterprise"}}, nil)
	_, err := suite.service.optionID(suite.Ctx, suite.actor, customer_repo.Tiers, "Startup")
	suite.Equal(`No customer tier "Startup". Options: Growth, Enterprise.`, errdefs.As(err).Message)

	suite.MockCustomerRepo.EXPECT().Named(mock.Anything, customer_repo.Statuses, suite.actor.Workspace).Return(nil, nil)
	_, err = suite.service.optionID(suite.Ctx, suite.actor, customer_repo.Statuses, "Active")
	suite.Equal(`No customer status "Active". There are none.`, errdefs.As(err).Message)
}

func (suite *CustomerServiceSuite) TestOptionNamesMatchInAnyCase() {
	growth := uuid.New()
	suite.MockCustomerRepo.EXPECT().Named(mock.Anything, customer_repo.Tiers, suite.actor.Workspace).
		Return([]customer_repo.Option{{ID: growth, Name: "Growth"}}, nil)
	found, err := suite.service.optionID(suite.Ctx, suite.actor, customer_repo.Tiers, " growth ")
	suite.NoError(err)
	suite.Equal(growth, found)
}

func (suite *CustomerServiceSuite) TestRevenueAndSizeCantBeNegative() {
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Name: new("Globex"), Revenue: args.Nullable[int64]{Set: true, Valid: true, Value: -1}})
	suite.Equal("revenue can't be negative", errdefs.As(err).Message)
	_, err = suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Name: new("Globex"), Size: args.Nullable[int32]{Set: true, Valid: true, Value: -5}})
	suite.Equal("size can't be negative", errdefs.As(err).Message)
}

func (suite *CustomerServiceSuite) TestNeedsAreForAnIssueOrAProject() {
	_, err := suite.service.SaveNeed(suite.Ctx, suite.actor, &SaveNeedInput{Customer: new("Globex"), Issue: new("ENG-1"), Project: new("Web")})
	suite.Equal("A need is for an issue or a project, not both", errdefs.As(err).Message)
	_, err = suite.service.SaveNeed(suite.Ctx, suite.actor, &SaveNeedInput{Customer: new("Globex")})
	suite.Equal("Pass the issue or the project the customer needs", errdefs.As(err).Message)
	_, err = suite.service.SaveNeed(suite.Ctx, suite.actor, &SaveNeedInput{Issue: new("ENG-1")})
	suite.Equal("customer is required to create a need", errdefs.As(err).Message)
}

func (suite *CustomerServiceSuite) TestBlankTextsBecomeNull() {
	suite.Equal(args.Nullable[string]{Set: true}, trimNullable(args.Nullable[string]{Set: true, Valid: true, Value: "  "}))
	suite.Equal(args.Nullable[string]{Set: true, Valid: true, Value: "x"}, trimNullable(args.Nullable[string]{Set: true, Valid: true, Value: " x "}))
	suite.Equal(args.Nullable[string]{}, trimNullable(args.Nullable[string]{}))
}
