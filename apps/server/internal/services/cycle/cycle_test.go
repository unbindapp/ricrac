package cycle_service

import (
	"context"
	"testing"
	"time"

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

type CycleServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	service *CycleService
	actor   *auth.Actor
	team    uuid.UUID
}

func (suite *CycleServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.service = NewCycleService(config.Config{}, suite.MockRepo, suite.resolve, mocks_services_events.NewEventsServiceMock(suite.T()))
	suite.team = uuid.New()
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return []uuid.UUID{suite.team}, nil })
	suite.ExpectTx()
}

func TestCycleService(t *testing.T) {
	suite.Run(t, new(CycleServiceSuite))
}

func (suite *CycleServiceSuite) TestFirstCycleStartsOnTheStartDay() {
	thursday := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	cases := map[int]string{1: "2026-10-12T00:00:00+00:00", 4: "2026-10-15T00:00:00+00:00", 5: "2026-10-09T00:00:00+00:00"}
	for day, want := range cases {
		suite.Equal(want, chronoRFC3339(firstStart(thursday, day)), "start day %d", day)
	}
}

func (suite *CycleServiceSuite) TestCreatingNeedsDates() {
	suite.resolve.EXPECT().TeamOrDefault(mock.Anything, suite.actor, (*string)(nil)).Return(suite.team, nil)
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{StartsAt: new("2026-10-12")})
	suite.Equal("startsAt and endsAt are required to create a cycle", errdefs.As(err).Message)
}

func (suite *CycleServiceSuite) TestTeamsWithoutCyclesGetNone() {
	suite.resolve.EXPECT().TeamOrDefault(mock.Anything, suite.actor, mock.Anything).Return(suite.team, nil)
	suite.MockCycleRepo.EXPECT().TeamUsesCycles(mock.Anything, mock.Anything, suite.team).Return(false, nil)
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{Team: new("ENG"), StartsAt: new("2026-10-12"), EndsAt: new("2026-10-26")})
	suite.Equal("This team doesn't use cycles; turn them on with save_team (cyclesEnabled) first", errdefs.As(err).Message)
}

func (suite *CycleServiceSuite) TestOverlapsNameTheCycle() {
	starts := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	ends := starts.Add(14 * 24 * time.Hour)
	err := suite.service.checkNoOverlap(suite.Ctx, nil, suite.team, nil, ends, starts)
	suite.Equal("endsAt must be after startsAt", errdefs.As(err).Message)

	clash := &ent.Cycle{Number: 3, StartsAt: starts.Add(-7 * 24 * time.Hour), EndsAt: starts.Add(7*24*time.Hour + 1500*time.Millisecond)}
	suite.MockCycleRepo.EXPECT().Overlapping(mock.Anything, mock.Anything, suite.team, (*uuid.UUID)(nil), starts, ends).Return(clash, nil)
	err = suite.service.checkNoOverlap(suite.Ctx, nil, suite.team, nil, starts, ends)
	suite.Equal("Those dates overlap cycle 3 (2026-10-05T00:00:00+00:00 to 2026-10-19T00:00:01.500+00:00); a team's cycles can't overlap",
		errdefs.As(err).Message)
}

func (suite *CycleServiceSuite) TestTheCurrentCycleCantBeArchived() {
	err := suite.service.archive(suite.Ctx, nil, suite.actor, uuid.New(), false, false, time.Now())
	suite.Equal("The current cycle can't be archived; wait until it is completed", errdefs.As(err).Message)
}
