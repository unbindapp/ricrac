package webhook_service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	webhook_repo "ricrac/internal/repositories/webhook"
)

// backoffMinutes is the wait before the 2nd, 3rd, ... attempt. After the last one the delivery is
// given up.
var backoffMinutes = []int{1, 5, 30, 120, 360}

type sender struct {
	client *http.Client
}

// newSender waits at most 10 seconds for an answer and doesn't follow redirects.
func newSender() *sender {
	return &sender{client: &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// signature is the hex HMAC-SHA256 of body keyed with secret, sent as Ricrac-Signature.
func signature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func truncateError(text string) *string {
	runes := []rune(text)
	return new(string(runes[:min(len(runes), 500)]))
}

func (self *sender) send(ctx context.Context, due webhook_repo.Due, payload models.M) webhook_repo.Outcome {
	payload = maps.Clone(payload)
	payload["webhookTimestamp"] = time.Now().UnixMilli()
	body := errdefs.Marshal(payload)
	event, _ := payload["type"].(string)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, due.URL, bytes.NewReader(body))
	if err != nil {
		return webhook_repo.Outcome{Error: truncateError(err.Error())}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Ricrac-Webhook/1.0")
	request.Header.Set("Ricrac-Signature", signature(due.Secret, body))
	request.Header.Set("Ricrac-Event", event)
	request.Header.Set("Ricrac-Delivery", due.ID.String())
	response, err := self.client.Do(request)
	if err != nil {
		return webhook_repo.Outcome{Error: truncateError(err.Error())}
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status >= 200 && status < 300 {
		return webhook_repo.Outcome{OK: true, Status: &status}
	}
	text, _ := io.ReadAll(response.Body)
	message := "HTTP " + response.Status
	if trimmed := strings.TrimSpace(string(text)); trimmed != "" {
		message += ": " + trimmed
	}
	return webhook_repo.Outcome{Status: &status, Error: truncateError(message)}
}

// record saves the result of an attempt: delivered, retried later with backoff, or given up.
func (self *WebhookService) record(ctx context.Context, due webhook_repo.Due, outcome webhook_repo.Outcome, retry bool) error {
	attempts := due.Attempts + 1
	now, err := self.repo.Now(ctx, nil)
	if err != nil {
		return err
	}
	if outcome.OK {
		return self.webhooks().MarkDelivered(ctx, due.ID, outcome.Status, attempts, now)
	}
	var retryAt *time.Time
	if retry && attempts <= len(backoffMinutes) {
		retryAt = new(now.Add(time.Duration(backoffMinutes[attempts-1]) * time.Minute))
	}
	return self.webhooks().MarkFailed(ctx, due.ID, outcome, attempts, retryAt, now)
}

// DeliverDue sends queued webhook deliveries that are due, retrying failures with backoff.
// Returns how many were attempted.
func (self *WebhookService) DeliverDue(ctx context.Context) (int, error) {
	var due []webhook_repo.Due
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.webhooks().GiveUpDisabled(ctx, tx, now); err != nil {
			return err
		}
		due, err = self.webhooks().ClaimDue(ctx, tx, 20, now)
		return err
	})
	if err != nil {
		return 0, err
	}
	due = slices.DeleteFunc(due, func(d webhook_repo.Due) bool { return !d.Enabled })
	errs := make([]error, len(due))
	var wg sync.WaitGroup
	for i, delivery := range due {
		wg.Go(func() {
			var payload models.M
			if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
				errs[i] = err
				return
			}
			outcome := self.sender.send(ctx, delivery, payload)
			errs[i] = self.record(ctx, delivery, outcome, true)
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return 0, err
	}
	return len(due), nil
}
