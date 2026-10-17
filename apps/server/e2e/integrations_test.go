package e2e

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type hookRequest struct {
	header http.Header
	body   []byte
}

// receiver is a local HTTP server answering every POST to /hook with status, recording what it got.
type receiver struct {
	url      string
	mu       sync.Mutex
	requests []hookRequest
}

func newReceiver(world *World, status int) *receiver {
	t := world.t
	r := &receiver{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.requests = append(r.requests, hookRequest{req.Header.Clone(), body})
		r.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	r.url = server.URL + "/hook"
	return r
}

func (r *receiver) pop(t *testing.T) hookRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) == 0 {
		t.Fatal("the receiver got nothing")
	}
	last := r.requests[len(r.requests)-1]
	r.requests = r.requests[:len(r.requests)-1]
	return last
}

func (r *receiver) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

func verifies(secret string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected, err := hex.DecodeString(signature)
	return err == nil && hmac.Equal(mac.Sum(nil), expected)
}

func deliverDue(t *testing.T, w *World) int {
	t.Helper()
	sent, err := w.B.DeliverDueWebhooks(context.Background())
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	return sent
}

func TestDeliversSignedWebhooks(t *testing.T) {
	w := world(t)
	hooks := newReceiver(w, http.StatusOK)
	hook := call(t, w.Ann, "save_webhook", M{"url": hooks.url, "label": "CI", "resourceTypes": []any{"issue"}})
	secret := hook.At("secret").Str()
	check(t, strings.HasPrefix(secret, "rr_whs_"))
	eqList(t, hook.At("resourceTypes").Strings(), []string{"Issue"})
	id := hook.At("id").Str()
	_, hasSecret := call(t, w.Ann, "get_webhook", M{"id": id}).V.(map[string]any)["secret"]
	check(t, !hasSecret, "the secret is never shown again")

	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Ship it"})
	eq(t, deliverDue(t, w), 1)
	request := hooks.pop(t)
	var payload J
	if err := json.Unmarshal(request.body, &payload.V); err != nil {
		t.Fatal(err)
	}
	eq(t, payload.At("action").Str(), "create")
	eq(t, payload.At("type").Str(), "Issue")
	eq(t, payload.At("data", "identifier").Str(), "ENG-1")
	check(t, payload.At("webhookTimestamp").Num() > 0)
	eq(t, request.header.Get("Ricrac-Event"), "Issue")
	eq(t, request.header.Get("User-Agent"), "Ricrac-Webhook/1.0")
	check(t, verifies(secret, request.body, request.header.Get("Ricrac-Signature")))
	check(t, !verifies("rr_whs_wrong", request.body, request.header.Get("Ricrac-Signature")))

	read := call(t, w.Ann, "get_webhook", M{"id": id})
	delivery := read.At("deliveries", 0)
	eq(t, delivery.At("action").Str(), "create")
	eq(t, delivery.At("statusCode").Num(), 200)
	eq(t, delivery.At("attempts").Num(), 1)
	check(t, delivery.At("deliveredAt").IsString())
	eq(t, request.header.Get("Ricrac-Delivery"), delivery.At("id").Str())
	eq(t, deliverDue(t, w), 0, "nothing is left to send")

	rotated := call(t, w.Ann, "rotate_webhook_secret", M{"id": id})
	newSecret := rotated.At("secret").Str()
	check(t, newSecret != secret)
	ping := call(t, w.Ann, "test_webhook", M{"id": id})
	eq(t, ping.At("delivered").Bool(), true)
	eq(t, ping.At("statusCode").Num(), 200)
	request = hooks.pop(t)
	if err := json.Unmarshal(request.body, &payload.V); err != nil {
		t.Fatal(err)
	}
	eq(t, payload.At("action").Str()+" "+payload.At("type").Str(), "test Ping")
	check(t, verifies(newSecret, request.body, request.header.Get("Ricrac-Signature")))

	commentsOnly := call(t, w.Ann, "save_webhook", M{"id": id, "resourceTypes": []any{"Comment"}})
	eqList(t, commentsOnly.At("resourceTypes").Strings(), []string{"Comment"})
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Unheard"})
	eq(t, deliverDue(t, w), 0, "the webhook no longer listens to issues")
}

func TestRetriesFailedDeliveries(t *testing.T) {
	w := world(t)
	hooks := newReceiver(w, http.StatusInternalServerError)
	hook := call(t, w.Ann, "save_webhook", M{"url": hooks.url, "resourceTypes": []any{"Issue"}, "team": "ENG"})
	eq(t, hook.At("team", "key").Str(), "ENG")
	id := hook.At("id").Str()
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Flaky"})
	eq(t, deliverDue(t, w), 1)
	eq(t, hooks.count(), 1)
	deliveries := call(t, w.Ann, "list_webhook_deliveries", M{"id": id})
	delivery := deliveries.At("nodes", 0)
	eq(t, delivery.At("attempts").Num(), 1)
	eq(t, delivery.At("statusCode").Num(), 500)
	contains(t, delivery.At("error").Str(), "500")
	check(t, delivery.At("nextAttemptAt").IsString() && delivery.At("failedAt").IsNull())
	eq(t, deliverDue(t, w), 0, "the retry waits a minute")

	ping := call(t, w.Ann, "test_webhook", M{"id": id})
	check(t, !ping.At("delivered").Bool() && ping.At("statusCode").Num() == 500)

	w.exec("UPDATE webhook_deliveries SET next_attempt_at = now()")
	call(t, w.Ann, "save_webhook", M{"id": id, "enabled": false})
	eq(t, deliverDue(t, w), 0)
	deliveries = call(t, w.Ann, "list_webhook_deliveries", M{"id": id})
	for _, d := range deliveries.At("nodes").List() {
		check(t, d.At("failedAt").IsString(), "every delivery is given up")
	}
	eq(t, hooks.count(), 2, "only the first attempt and the ping were sent")
}

func TestOnlyAdminsManageWebhooks(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	contains(t, fails(t, bob, "list_webhooks", M{}), "admin")
	contains(t, fails(t, w.Ann, "save_webhook", M{"url": "http://hooks.example.com/x"}), "https")
	kind := fails(t, w.Ann, "save_webhook", M{"url": "https://hooks.example.com", "resourceTypes": []any{"Bug"}})
	contains(t, kind, "CustomerNeed")
	hook := call(t, w.Ann, "save_webhook", M{"url": "https://hooks.example.com"})
	eqList(t, hook.At("resourceTypes").Strings(), []string{"Issue", "Comment"})
	eq(t, call(t, w.Ann, "list_webhooks", M{}).At("nodes").Len(), 1)
	call(t, w.Ann, "delete_webhook", M{"id": hook.At("id").V})
	eq(t, call(t, w.Ann, "list_webhooks", M{}).At("nodes").Len(), 0)
}

func TestCustomersWithNeedsStatusesAndTiers(t *testing.T) {
	w := world(t)
	statuses := call(t, w.Ann, "list_customer_statuses", M{})
	eq(t, statuses.At("nodes", 0, "name").Str(), "Active")
	call(t, w.Ann, "save_customer_tier", M{"name": "Enterprise", "color": "#5E6AD2"})
	globex := call(t, w.Ann, "save_customer", M{"name": "Globex", "domains": []any{"https://Globex.com/"}, "status": "active",
		"tier": "enterprise", "owner": "me", "revenue": 50000, "size": 200, "externalIds": []any{"crm-42"}})
	eqList(t, globex.At("domains").Strings(), []string{"globex.com"})
	eq(t, globex.At("status", "name").Str(), "Active")
	eq(t, globex.At("tier", "color").Str(), "#5e6ad2")
	eq(t, globex.At("owner", "email").Str(), "ann@acme.test")
	contains(t, fails(t, w.Ann, "save_customer", M{"name": "Initech", "domains": []any{"globex.com"}}), "Globex")

	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "SSO"})
	need := call(t, w.Ann, "save_customer_need", M{"customer": "globex.com", "issue": "ENG-1", "body": "We need SSO", "priority": 1})
	eq(t, need.At("customer", "name").Str(), "Globex")
	eq(t, need.At("issue", "identifier").Str(), "ENG-1")
	contains(t, fails(t, w.Ann, "save_customer_need", M{"customer": "Globex", "body": "?"}), "issue or the project")
	contains(t, fails(t, w.Ann, "save_customer_need", M{"customer": "Globex", "issue": "ENG-9"}), "ENG-9")

	read := call(t, w.Ann, "get_customer", M{"id": "crm-42"})
	eq(t, read.At("needCount").Num(), 1)
	eq(t, read.At("needs", 0, "body").Str(), "We need SSO")
	eq(t, read.At("needs", 0, "issue", "state", "name").Str(), "Backlog")
	byIssue := call(t, w.Ann, "list_customer_needs", M{"issue": "ENG-1"})
	eq(t, byIssue.At("nodes", 0, "priority").Num(), 1)
	eq(t, call(t, w.Ann, "list_customers", M{"query": "globex.c", "tier": "Enterprise"}).At("nodes").Len(), 1)
	eq(t, call(t, w.Ann, "list_customers", M{"status": "Churned"}).At("nodes").Len(), 0)

	cleared := call(t, w.Ann, "delete_customer_tier", M{"id": "Enterprise"})
	eq(t, cleared.At("customersCleared").Num(), 1)
	updated := call(t, w.Ann, "save_customer", M{"id": "Globex", "status": nil, "revenue": nil})
	check(t, updated.At("tier").IsNull() && updated.At("status").IsNull() && updated.At("revenue").IsNull())
	eq(t, updated.At("size").Num(), 200)

	archived := call(t, w.Ann, "delete_customer", M{"id": "Globex"})
	check(t, archived.At("archivedAt").IsString())
	eq(t, call(t, w.Ann, "list_customers", M{}).At("nodes").Len(), 0)
	eq(t, call(t, w.Ann, "list_customers", M{"includeArchived": true}).At("nodes", 0, "name").Str(), "Globex")
	eq(t, call(t, w.Ann, "list_customer_needs", M{"issue": "ENG-1"}).At("nodes").Len(), 1)
	call(t, w.Ann, "delete_customer_need", M{"id": need.At("id").V})
	eq(t, call(t, w.Ann, "list_customer_needs", M{}).At("nodes").Len(), 0)
}

func TestManagesCustomerStatusesAndTiers(t *testing.T) {
	w := world(t)
	trial := call(t, w.Ann, "save_customer_status", M{"name": "Trial", "description": "Trying it out"})
	eq(t, trial.At("description").Str(), "Trying it out")
	eq(t, trial.At("customerCount").Num(), 0)
	call(t, w.Ann, "save_customer", M{"name": "Globex", "status": "Trial"})
	renamed := call(t, w.Ann, "save_customer_status", M{"id": "Trial", "name": "Pilot", "description": nil})
	check(t, renamed.At("description").IsNull())
	eq(t, renamed.At("customerCount").Num(), 1)
	eq(t, call(t, w.Ann, "delete_customer_status", M{"id": "Pilot"}).At("customersCleared").Num(), 1)
	check(t, call(t, w.Ann, "get_customer", M{"id": "Globex"}).At("status").IsNull())

	call(t, w.Ann, "save_customer_tier", M{"name": "Growth", "position": 2})
	call(t, w.Ann, "save_customer_tier", M{"name": "Enterprise", "position": 1})
	eqList(t, names(call(t, w.Ann, "list_customer_tiers", M{})), []string{"Enterprise", "Growth"})
}

func names(list J) []string {
	return list.At("nodes").Strings("name")
}

func TestReleasesMoveThroughStagesWithIssuesAndNotes(t *testing.T) {
	w := world(t)
	pipeline := call(t, w.Ann, "save_release_pipeline", M{"name": "Web", "type": "scheduled", "isProduction": true, "addTeams": []any{"ENG"}})
	eqList(t, pipeline.At("stages").Strings("name"), []string{"Planned", "In Progress", "Released", "Canceled"})
	eq(t, pipeline.At("teams", 0, "key").Str(), "ENG")
	listed := call(t, w.Ann, "list_release_pipelines", M{"team": "ENG", "includeStages": true})
	eq(t, listed.At("nodes", 0, "stages").Len(), 4)

	spring := call(t, w.Ann, "save_release", M{"name": "Spring", "pipeline": "Web", "version": "1.0", "createdAt": "2026-01-01T00:00:00Z"})
	eq(t, spring.At("stage", "name").Str(), "Planned")
	check(t, spring.At("startedAt").IsNull())
	started := call(t, w.Ann, "save_release", M{"id": "Spring", "stage": "started"})
	eq(t, started.At("stage", "name").Str(), "In Progress")
	check(t, started.At("startedAt").IsString() && started.At("completedAt").IsNull())
	released := call(t, w.Ann, "save_release", M{"id": spring.At("slugId").V, "stage": "Released"})
	check(t, released.At("completedAt").IsString())
	eq(t, released.At("startedAt").Str(), started.At("startedAt").Str())
	dated := call(t, w.Ann, "save_release", M{"id": "Spring", "completedAt": "2026-01-05T10:00:00Z", "targetDate": "2026-01-04"})
	check(t, strings.HasPrefix(dated.At("completedAt").Str(), "2026-01-05T10:00:00"))
	eq(t, dated.At("targetDate").Str(), "2026-01-04")

	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Login", "addReleases": []any{"Spring"}})
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Not shipped"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"release": "Spring"})), []string{"ENG-1"})
	read := call(t, w.Ann, "get_release", M{"id": "Spring"})
	eq(t, read.At("issueCount").Num(), 1)
	eq(t, read.At("issues", 0, "identifier").Str(), "ENG-1")
	eq(t, read.At("issues", 0, "state", "name").Str(), "Backlog")

	for _, r := range []struct{ name, version, month string }{{"Summer", "1.1", "2"}, {"Autumn", "1.2", "3"}, {"Winter", "1.3", "4"}} {
		call(t, w.Ann, "save_release", M{"name": r.name, "pipeline": "Web", "version": r.version, "createdAt": "2026-0" + r.month + "-01T00:00:00Z"})
	}
	note := call(t, w.Ann, "save_release_note", M{"pipeline": "Web", "rangeFromRelease": "Autumn", "rangeToRelease": "Summer", "content": "Faster pages"})
	eq(t, note.At("title").Str(), "Autumn (1.2)")
	eqList(t, note.At("releases").Strings("name"), []string{"Summer", "Autumn"})
	patched := call(t, w.Ann, "save_release_note", M{"id": note.At("slugId").V, "patch": []any{M{"op": "append", "text": "\nAnd fixes"}}})
	eq(t, patched.At("content").Str(), "Faster pages\nAnd fixes")
	eq(t, call(t, w.Ann, "get_release", M{"id": "Autumn"}).At("releaseNotes", 0, "title").Str(), "Autumn (1.2)")
	eq(t, call(t, w.Ann, "list_release_notes", M{"pipeline": "Web"}).At("nodes", 0, "title").Str(), "Autumn (1.2)")

	eqList(t, names(call(t, w.Ann, "list_releases", M{"pipeline": "Web", "stage": "Released"})), []string{"Spring"})
	eqList(t, names(call(t, w.Ann, "list_releases", M{"stage": "planned"})), []string{"Winter", "Autumn", "Summer"})
	contains(t, fails(t, w.Ann, "list_releases", M{"stage": "Released"}), "pipeline")

	call(t, w.Ann, "save_release_stage", M{"pipeline": "Web", "name": "QA", "type": "started"})
	planned := pipeline.At("stages", 0, "id").Str()
	contains(t, fails(t, w.Ann, "delete_release_stage", M{"id": planned}), "moveReleasesTo")
	deleted := call(t, w.Ann, "delete_release_stage", M{"id": planned, "moveReleasesTo": "QA"})
	eq(t, deleted.At("releasesMoved").Num(), 3)
	eq(t, call(t, w.Ann, "get_release", M{"id": "Winter"}).At("stage", "name").Str(), "QA")

	call(t, w.Ann, "delete_release", M{"id": "Spring"})
	eqList(t, names(call(t, w.Ann, "list_releases", M{"pipeline": "Web"})), []string{"Winter", "Autumn", "Summer"})

	read = call(t, w.Ann, "get_release_note", M{"id": note.At("id").V})
	eq(t, read.At("content").Str(), "Faster pages\nAnd fixes")
	call(t, w.Ann, "delete_release_note", M{"id": note.At("slugId").V})
	eq(t, call(t, w.Ann, "list_release_notes", M{}).At("nodes").Len(), 0)

	read = call(t, w.Ann, "get_release_pipeline", M{"id": "Web"})
	eqList(t, read.At("stages").Strings("name"), []string{"In Progress", "Released", "Canceled", "QA"})
	eq(t, read.At("releaseCount").Num(), 3)
	archived := call(t, w.Ann, "delete_release_pipeline", M{"id": "Web"})
	check(t, archived.At("archivedAt").IsString())
	eq(t, call(t, w.Ann, "list_release_pipelines", M{}).At("nodes").Len(), 0)
}
