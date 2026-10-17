package release_service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	resolve_repo "ricrac/internal/repositories/resolve"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type ReleaseServiceSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	events  *mocks_services_events.EventsServiceMock
	service *ReleaseService
	actor   *auth.Actor
}

func (suite *ReleaseServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	suite.events = mocks_services_events.NewEventsServiceMock(suite.T())
	suite.service = NewReleaseService(suite.MockRepo, suite.resolve, suite.events)
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
	suite.ExpectTx()
}

func TestReleaseService(t *testing.T) {
	suite.Run(t, new(ReleaseServiceSuite))
}

func (suite *ReleaseServiceSuite) TestNotesAreFoundByIDSlugOrLink() {
	id := uuid.New().String()
	suite.Equal(id, idOrSlug(id))
	suite.Equal("0123456789ab", idOrSlug("https://ricrac.app/acme/notes/spring-release-0123456789ab/"))
	suite.Equal("spring-release", idOrSlug("spring-release"))
	suite.Equal(id, idOrSlug("https://ricrac.app/acme/notes/"+id))
}

func (suite *ReleaseServiceSuite) TestReleasesOrderOnlyByKnownFields() {
	_, err := suite.service.ListReleases(suite.Ctx, suite.actor, &ListReleasesInput{OrderBy: new("name")})
	suite.Equal(`orderBy "name" isn't one of createdAt, updatedAt, targetDate`, errdefs.As(err).Message)
}

func (suite *ReleaseServiceSuite) TestStageNamesNeedThePipeline() {
	_, err := suite.service.ListReleases(suite.Ctx, suite.actor, &ListReleasesInput{Stage: new("Released")})
	suite.Equal("Name the pipeline too to filter by stage name, or pass a stage type or id", errdefs.As(err).Message)
}

func (suite *ReleaseServiceSuite) TestTeamsAreSetOrChangedNotBoth() {
	_, err := suite.service.SavePipeline(suite.Ctx, suite.actor, &SavePipelineInput{SetTeams: []string{"ENG"}, RemoveTeams: []string{"OPS"}})
	suite.Equal("Pass setTeams or addTeams/removeTeams, not both", errdefs.As(err).Message)
}

func (suite *ReleaseServiceSuite) TestStagesAreFoundByNameOrType() {
	pipeline, released := uuid.New(), uuid.New()
	rows := []resolve_repo.Typed{{ID: uuid.New(), Name: "Planned", Type: "planned"}, {ID: released, Name: "Released", Type: "completed"}}
	suite.MockReleaseRepo.EXPECT().Stages(mock.Anything, mock.Anything, pipeline).Return(rows, nil)
	found, err := suite.service.stage(suite.Ctx, nil, pipeline, "completed")
	suite.NoError(err)
	suite.Equal(released, found)
	_, err = suite.service.stage(suite.Ctx, nil, pipeline, "QA")
	suite.Equal(`The pipeline has no stage "QA". Options: Planned (planned), Released (completed).`, errdefs.As(err).Message)
}

func (suite *ReleaseServiceSuite) TestNotesAreNamedAfterTheNewestRelease() {
	ids := []uuid.UUID{uuid.New()}
	suite.MockReleaseRepo.EXPECT().Newest(mock.Anything, mock.Anything, ids).Return(&ent.Release{Name: "Autumn", Version: new("1.2")}, nil).Once()
	title, err := suite.service.defaultNoteTitle(suite.Ctx, nil, ids)
	suite.NoError(err)
	suite.Equal("Autumn (1.2)", *title)

	suite.MockReleaseRepo.EXPECT().Newest(mock.Anything, mock.Anything, ids).Return(&ent.Release{Name: "Release 1.3", Version: new("1.3")}, nil).Once()
	title, err = suite.service.defaultNoteTitle(suite.Ctx, nil, ids)
	suite.NoError(err)
	suite.Equal("Release 1.3", *title)
}

func (suite *ReleaseServiceSuite) TestRangesNeedBothEnds() {
	_, err := suite.service.coveredReleases(suite.Ctx, suite.actor, uuid.New(), &SaveNoteInput{RangeFromRelease: new("Spring")})
	suite.Equal("Pass both rangeFromRelease and rangeToRelease", errdefs.As(err).Message)
}
