package webhook_service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	webhook_repo "ricrac/internal/repositories/webhook"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type WebhookServiceSuite struct {
	services.ServiceTestSuite
	service *WebhookService
	actor   *auth.Actor
	now     time.Time
}

func (suite *WebhookServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.service = NewWebhookService(suite.MockRepo, mocks_services_resolve.NewResolveServiceMock(suite.T()), mocks_services_events.NewEventsServiceMock(suite.T()))
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Admin, auth.EveryAccess, nil,
		func(context.Context, *auth.Actor) ([]uuid.UUID, error) { return nil, nil })
	suite.now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	suite.MockRepo.EXPECT().Now(mock.Anything, mock.Anything).Return(suite.now, nil).Maybe()
}

func TestWebhookService(t *testing.T) {
	suite.Run(t, new(WebhookServiceSuite))
}

func (suite *WebhookServiceSuite) TestURLsAreHTTPSOrLocal() {
	for _, value := range []string{"https://hooks.example.com/x", " http://localhost:8080/hook ", "http://127.0.0.1/hook"} {
		_, err := checkURL(value)
		suite.NoError(err, value)
	}
	for _, value := range []string{"http://hooks.example.com/x", "https://", "ftp://localhost/x"} {
		_, err := checkURL(value)
		suite.Contains(errdefs.As(err).Message, "isn't a valid webhook URL", value)
	}
}

func (suite *WebhookServiceSuite) TestResourceTypesAreNamedInAnyCaseOnce() {
	types, err := checkResourceTypes([]string{"issue", " Comment ", "ISSUE"})
	suite.NoError(err)
	suite.Equal([]string{"Issue", "Comment"}, types)
	_, err = checkResourceTypes([]string{})
	suite.Equal("Pass at least one resource type", errdefs.As(err).Message)
	_, err = checkResourceTypes([]string{"Bug"})
	suite.Contains(errdefs.As(err).Message, `"Bug" isn't a resource type. Options: Issue, Comment,`)
}

func (suite *WebhookServiceSuite) TestNewWebhooksNeedAURL() {
	_, err := suite.service.Save(suite.Ctx, suite.actor, &SaveInput{ResourceTypes: []string{"Issue"}})
	suite.Equal("url is required to create a webhook", errdefs.As(err).Message)
}

func (suite *WebhookServiceSuite) TestFailuresAreRetriedWithBackoffThenGivenUp() {
	delivery := uuid.New()
	failed := webhook_repo.Outcome{Status: new(500), Error: new("HTTP 500 Internal Server Error")}
	retryAt := suite.now.Add(30 * time.Minute)
	suite.MockWebhookRepo.EXPECT().MarkFailed(mock.Anything, delivery, failed, 3, &retryAt, suite.now).Return(nil).Once()
	suite.NoError(suite.service.record(suite.Ctx, webhook_repo.Due{ID: delivery, Attempts: 2}, failed, true))

	suite.MockWebhookRepo.EXPECT().MarkFailed(mock.Anything, delivery, failed, 6, (*time.Time)(nil), suite.now).Return(nil).Once()
	suite.NoError(suite.service.record(suite.Ctx, webhook_repo.Due{ID: delivery, Attempts: 5}, failed, true))

	suite.MockWebhookRepo.EXPECT().MarkFailed(mock.Anything, delivery, failed, 1, (*time.Time)(nil), suite.now).Return(nil).Once()
	suite.NoError(suite.service.record(suite.Ctx, webhook_repo.Due{ID: delivery}, failed, false), "pings aren't retried")
}

func (suite *WebhookServiceSuite) TestDeliveriesAreSignedAndReportWhatTheReceiverSaid() {
	var header http.Header
	var body []byte
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header, body = r.Header.Clone(), must(io.ReadAll(r.Body))
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(" not today \n"))
	}))
	defer receiver.Close()
	due := webhook_repo.Due{ID: uuid.New(), URL: receiver.URL, Secret: "rr_whs_test"}
	outcome := suite.service.sender.send(suite.Ctx, due, models.M{"type": "Issue", "action": "<create>"})

	suite.False(outcome.OK)
	suite.Equal(418, *outcome.Status)
	suite.Equal("HTTP 418 I'm a teapot: not today", *outcome.Error)
	suite.Equal("Issue", header.Get("Ricrac-Event"))
	suite.Equal(due.ID.String(), header.Get("Ricrac-Delivery"))
	suite.Equal(signature("rr_whs_test", body), header.Get("Ricrac-Signature"))
	suite.Contains(string(body), `"action":"<create>"`)
	var sent models.M
	suite.NoError(json.Unmarshal(body, &sent))
	suite.Greater(sent["webhookTimestamp"], float64(0))
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
