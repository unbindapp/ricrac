package account_handler

import (
	"fmt"
	"net/http"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	assistant_service "ricrac/internal/services/assistant"
)

// ask answers a question to the assistant as server-sent events.
func (h *handlers) ask(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	var body assistant_service.AskInput
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	question, err := h.assistant.Ask(r.Context(), actor, &body)
	if err != nil {
		return err
	}
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	stream := http.NewResponseController(w)
	question.Answer(r.Context(), func(event string, data any) error {
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, errdefs.Marshal(data)); err != nil {
			return err
		}
		return stream.Flush()
	})
	return nil
}

func (h *handlers) assistantSettings(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	settings, err := h.assistant.Settings(r.Context(), actor)
	if err != nil {
		return err
	}
	return answer(w, settings)
}

func (h *handlers) saveAssistantSettings(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	var body assistant_service.AssistantSettingsInput
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	settings, err := h.assistant.SaveSettings(r.Context(), actor, &body)
	if err != nil {
		return err
	}
	return answer(w, settings)
}
