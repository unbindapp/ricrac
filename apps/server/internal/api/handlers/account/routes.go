package account_handler

import (
	"net/http"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	account_service "ricrac/internal/services/account"
	oauth_service "ricrac/internal/services/oauth"
)

func (h *handlers) config(w http.ResponseWriter, r *http.Request) {
	errdefs.WriteJSON(w, http.StatusOK, h.account.Config())
}

// me is the signed-in user, or null for visitors.
func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	user, signedIn := h.keys.SessionUser(r)
	if !signedIn {
		errdefs.WriteJSON(w, http.StatusOK, nil)
		return
	}
	me, err := h.account.Me(r.Context(), user)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	errdefs.WriteJSON(w, http.StatusOK, me)
}

func (h *handlers) updateMe(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body account_service.UpdateMe
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if err := h.account.UpdateMe(r.Context(), user, &body); err != nil {
		return err
	}
	return ok(w)
}

func (h *handlers) createWorkspace(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body account_service.CreateWorkspace
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	summary, err := h.account.CreateWorkspace(r.Context(), user, &body)
	if err != nil {
		return err
	}
	return answer(w, summary)
}

func (h *handlers) deleteWorkspace(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var body account_service.DeleteWorkspace
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if err := h.account.DeleteWorkspace(r.Context(), user, id, &body); err != nil {
		return err
	}
	return ok(w)
}

func (h *handlers) leaveWorkspace(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.account.LeaveWorkspace(r.Context(), user, id); err != nil {
		return err
	}
	return ok(w)
}

func (h *handlers) acceptInvitation(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	summary, err := h.account.AcceptInvitation(r.Context(), user, id)
	if err != nil {
		return err
	}
	return answer(w, summary)
}

func (h *handlers) declineInvitation(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.account.DeclineInvitation(r.Context(), user, id); err != nil {
		return err
	}
	return ok(w)
}

func (h *handlers) listAPIKeys(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	keys, err := h.account.APIKeys(r.Context(), user)
	if err != nil {
		return err
	}
	return answer(w, keys)
}

func (h *handlers) createAPIKey(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body account_service.CreateAPIKey
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	created, err := h.account.CreateAPIKey(r.Context(), user, &body)
	if err != nil {
		return err
	}
	return answer(w, created)
}

func (h *handlers) deleteAPIKey(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.account.DeleteAPIKey(r.Context(), user, id); err != nil {
		return err
	}
	return ok(w)
}

func (h *handlers) listOAuthApps(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	apps, err := h.account.OAuthApps(r.Context(), actor)
	if err != nil {
		return err
	}
	return answer(w, apps)
}

func (h *handlers) createOAuthApp(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	if err := actor.RequireMember(); err != nil {
		return err
	}
	var body account_service.SaveOAuthApp
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	created, err := h.account.CreateOAuthApp(r.Context(), actor, &body)
	if err != nil {
		return err
	}
	return answer(w, created)
}

func (h *handlers) updateOAuthApp(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	id := pathParam(r, "id")
	if err := h.account.CheckAppOwner(r.Context(), actor, id); err != nil {
		return err
	}
	var body account_service.SaveOAuthApp
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	updated, err := h.account.UpdateOAuthApp(r.Context(), actor, id, &body)
	if err != nil {
		return err
	}
	return answer(w, updated)
}

func (h *handlers) rotateOAuthSecret(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	rotated, err := h.account.RotateOAuthSecret(r.Context(), actor, pathParam(r, "id"))
	if err != nil {
		return err
	}
	return answer(w, rotated)
}

func (h *handlers) deleteOAuthApp(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error {
	if err := h.account.DeleteOAuthApp(r.Context(), actor, pathParam(r, "id")); err != nil {
		return err
	}
	return ok(w)
}

func (h *handlers) connectedApps(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	apps, err := h.account.ConnectedApps(r.Context(), user)
	if err != nil {
		return err
	}
	return answer(w, apps)
}

func (h *handlers) revokeConnectedApp(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.account.RevokeConnectedApp(r.Context(), user, id); err != nil {
		return err
	}
	return ok(w)
}

func (h *handlers) oauthClient(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	optional := func(key string) *string {
		if !query.Has(key) {
			return nil
		}
		value := query.Get(key)
		return &value
	}
	for _, key := range []string{"client_id", "redirect_uri", "response_type"} {
		if !query.Has(key) {
			errdefs.Write(w, errdefs.BadRequest("Missing %s", key))
			return
		}
	}
	info, err := h.oauth.CheckRequest(r.Context(), &oauth_service.OAuthRequest{
		ClientID:            query.Get("client_id"),
		RedirectURI:         query.Get("redirect_uri"),
		ResponseType:        query.Get("response_type"),
		CodeChallenge:       optional("code_challenge"),
		CodeChallengeMethod: optional("code_challenge_method"),
		State:               optional("state"),
		Scope:               optional("scope"),
		Resource:            optional("resource"),
	})
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	errdefs.WriteJSON(w, http.StatusOK, info)
}

func (h *handlers) oauthDecide(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body oauth_service.OAuthDecision
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	redirect, err := h.oauth.Decide(r.Context(), user, body.WorkspaceIDs, &body.Request, body.Allow)
	if err != nil {
		return err
	}
	return answer(w, models.OAuthRedirect{Redirect: redirect})
}
