package template_service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type TemplateServiceSuite struct {
	services.ServiceTestSuite
	service *TemplateService
	actor   *auth.Actor
	team    uuid.UUID
}

func (suite *TemplateServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.service = NewTemplateService(suite.MockRepo, mocks_services_resolve.NewResolveServiceMock(suite.T()),
		mocks_services_events.NewEventsServiceMock(suite.T()), nil, nil)
	suite.team = uuid.New()
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return []uuid.UUID{suite.team}, nil })
	suite.ExpectTx()
}

func TestTemplateService(t *testing.T) {
	suite.Run(t, new(TemplateServiceSuite))
}

func (suite *TemplateServiceSuite) TestDataOnlyHoldsWhatTheTypeFillsIn() {
	cases := map[string]struct {
		kind string
		data args.M
	}{
		"Invalid issue template data: unknown field `titel`, expected one of `title`, `description`, `team`, `state`, " +
			"`priority`, `estimate`, `assignee`, `labels`, `project`, `cycle`, `dueDate`, `subIssues`": {"issue", args.M{"titel": "x"}},
		"Invalid project template data: missing field `name`":                                  {"project", args.M{"milestones": []any{args.M{}}}},
		"Invalid issue template data: `subIssues.0.priority` should be an integer, not string": {"issue", args.M{"subIssues": []any{args.M{"priority": "high"}}}},
		"data must be an object": {"document", nil},
	}
	for want, c := range cases {
		var data any = c.data
		if c.data == nil {
			data = "text"
		}
		suite.Equal(want, errdefs.As(checkData(c.kind, data)).Message)
	}
	suite.NoError(checkData("issue", args.M{"subIssues": []any{args.M{"title": "Write tests", "estimate": nil}}}))
}

func (suite *TemplateServiceSuite) TestRecurrenceIsForIssueTemplates() {
	id := uuid.New()
	err := suite.service.setRecurrence(suite.Ctx, nil, id, "document", &Recurrence{Every: 1, Unit: "week"})
	suite.Equal("Only issue templates recur", errdefs.As(err).Message)
	err = suite.service.setRecurrence(suite.Ctx, nil, id, "issue", &Recurrence{Every: 1, Unit: "year"})
	suite.Equal("recurrence is {every: 1-365, unit: day|week|month, startsAt?}", errdefs.As(err).Message)

	starts := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	suite.MockTemplateRepo.EXPECT().
		SetRecurrence(mock.Anything, mock.Anything, id, json.RawMessage(`{"every":2,"startsAt":"2026-11-02T09:00:00Z","unit":"week"}`), &starts).
		Return(nil)
	suite.NoError(suite.service.setRecurrence(suite.Ctx, nil, id, "issue", &Recurrence{Every: 2, Unit: "week", StartsAt: new("2026-11-02T09:00:00Z")}))
}

func (suite *TemplateServiceSuite) TestRecurringTemplatesNeedATeam() {
	recurring := json.RawMessage(`{"every":1,"unit":"day"}`)
	suite.True(teamless(&ent.Template{Recurrence: recurring, Data: json.RawMessage(`{"title":"Standup"}`)}))
	suite.False(teamless(&ent.Template{Recurrence: recurring, Data: json.RawMessage(`{"team":"ENG"}`)}))
	suite.False(teamless(&ent.Template{Recurrence: recurring, TeamID: &suite.team}))
	suite.False(teamless(&ent.Template{Data: json.RawMessage(`{}`)}))
}

func (suite *TemplateServiceSuite) TestMonthsKeepTheDayUnlessTheMonthIsShorter() {
	at := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	suite.Equal(time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC), *nextRun(at, 1, "month"))
	suite.Equal(time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC), *nextRun(at, 2, "month"))
	suite.Equal(time.Date(2026, 2, 14, 9, 0, 0, 0, time.UTC), *nextRun(at, 2, "week"))
	suite.Nil(nextRun(at, 1, "year"))
}

func (suite *TemplateServiceSuite) TestSharedNamesNeedAType() {
	suite.MockTemplateRepo.EXPECT().Named(mock.Anything, suite.actor.Workspace, []uuid.UUID{suite.team}, (*uuid.UUID)(nil), "Bug").
		Return([]*ent.Template{{ID: uuid.New(), Type: "issue"}, {ID: uuid.New(), Type: "document"}}, nil)
	_, err := suite.service.find(suite.Ctx, suite.actor, nil, " Bug ")
	suite.Equal(`Several templates are named "Bug" (issue, document); pass type too`, errdefs.As(err).Message)
}

func (suite *TemplateServiceSuite) TestGuestsCantChangeTemplates() {
	guest := auth.NewActor(uuid.New(), suite.actor.Workspace, auth.Guest, auth.EveryAccess, nil, nil)
	_, err := suite.service.Delete(suite.Ctx, guest, &DeleteInput{ID: "Bug"})
	suite.Equal(403, errdefs.As(err).Status)
}
