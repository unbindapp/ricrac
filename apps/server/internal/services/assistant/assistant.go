package assistant_service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	"ricrac/internal/repositories/repositories"
	document_service "ricrac/internal/services/document"
	events_service "ricrac/internal/services/events"
	workspace_service "ricrac/internal/services/workspace"
)

const (
	questionTimeout = 10 * time.Minute
	guideDocument   = "Assistant"
	guideMaxLength  = 20000
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AssistantServiceInterface -p assistant_service -s AssistantService -o assistant_service_iface.go
type AssistantService struct {
	cfg        config.Config
	keys       auth.Keys
	repo       repositories.RepositoriesInterface
	workspaces workspace_service.WorkspaceServiceInterface
	documents  document_service.DocumentServiceInterface
	events     events_service.EventsServiceInterface
	mu         sync.Mutex
	asking     map[uuid.UUID]bool
}

func NewAssistantService(cfg config.Config, keys auth.Keys, repo repositories.RepositoriesInterface,
	workspaces workspace_service.WorkspaceServiceInterface, documents document_service.DocumentServiceInterface,
	events events_service.EventsServiceInterface) *AssistantService {
	return &AssistantService{cfg: cfg, keys: keys, repo: repo, workspaces: workspaces, documents: documents, events: events,
		asking: map[uuid.UUID]bool{}}
}

// Question is a question the assistant took: Answer runs it.
type Question struct {
	service *AssistantService
	actor   *auth.Actor
	session uuid.UUID
	resume  bool
	prompt  string
	system  string
	budget  float64
}

// Ask checks the question can run (the assistant is on in the workspace and the caller is picked
// to use it, the conversation is theirs, the budget isn't spent and they have no other question
// running) and takes it.
func (self *AssistantService) Ask(ctx context.Context, actor *auth.Actor, input *AskInput) (*Question, error) {
	settings := self.cfg.Assistant
	if settings == nil {
		return nil, errdefs.NotFound("The assistant is off: set ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN on the server")
	}
	if actor.Role == auth.Guest {
		return nil, errdefs.Forbidden("Guests can't use the assistant")
	}
	workspace, err := self.workspaces.Get(ctx, actor, nil)
	if err != nil {
		return nil, err
	}
	user, err := self.repo.Account().User(ctx, actor.User)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errdefs.Unauthorized()
	}
	allowed, err := self.repo.Assistant().Allowed(ctx, actor.Workspace, actor.User)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errdefs.NotFound("Claude is off for you in this workspace; a workspace admin can turn it on in settings")
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, errdefs.BadRequest("Ask something")
	}
	session, resume, err := self.session(ctx, actor, input.SessionID)
	if err != nil {
		return nil, err
	}
	spent, err := self.repo.Assistant().SpentToday(ctx)
	if err != nil {
		return nil, err
	}
	remaining := settings.DailyBudgetUSD - spent
	if remaining < 0.01 {
		return nil, errdefs.Errorf(http.StatusTooManyRequests, "The assistant spent its $%.2f for the last 24 hours; try again later", settings.DailyBudgetUSD)
	}
	system, err := self.systemPrompt(ctx, actor, workspace, user)
	if err != nil {
		return nil, err
	}
	if !self.claim(actor.User) {
		return nil, errdefs.Errorf(http.StatusTooManyRequests, "You have a question running already; wait for its answer or stop it")
	}
	return &Question{
		service: self, actor: actor, session: session, resume: resume, system: system,
		prompt: withPage(prompt, input.Page),
		budget: min(settings.QuestionBudgetUSD, remaining),
	}, nil
}

// session is the conversation to continue, or a new one.
func (self *AssistantService) session(ctx context.Context, actor *auth.Actor, value *string) (uuid.UUID, bool, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return uuid.New(), false, nil
	}
	gone := errdefs.BadRequest("That conversation is gone; start a new one")
	session, err := uuid.Parse(strings.TrimSpace(*value))
	if err != nil {
		return uuid.Nil, false, gone
	}
	user, workspace, found, err := self.repo.Assistant().SessionOwner(ctx, session)
	if err != nil {
		return uuid.Nil, false, err
	}
	if !found || user != actor.User || workspace != actor.Workspace {
		return uuid.Nil, false, gone
	}
	return session, true, nil
}

func (self *AssistantService) claim(user uuid.UUID) bool {
	self.mu.Lock()
	defer self.mu.Unlock()
	if self.asking[user] {
		return false
	}
	self.asking[user] = true
	return true
}

func (self *AssistantService) release(user uuid.UUID) {
	self.mu.Lock()
	defer self.mu.Unlock()
	delete(self.asking, user)
}

func withPage(prompt string, page *string) string {
	if page == nil || strings.TrimSpace(*page) == "" {
		return prompt
	}
	return fmt.Sprintf("(I'm looking at %s)\n\n%s", strings.TrimSpace(*page), prompt)
}

func (self *AssistantService) systemPrompt(ctx context.Context, actor *auth.Actor, workspace models.Workspace, user *ent.User) (string, error) {
	prefix := "ENG"
	if len(workspace.Teams) > 0 {
		prefix = workspace.Teams[0].Key
	}
	var prompt strings.Builder
	fmt.Fprintf(&prompt, "You are Claude, the assistant in Ricrac, an issue tracker. You're helping %s (@%s) in the workspace %q "+
		"(URL key %s, issues numbered like %s-12). Today is %s.\n\n",
		user.Name, user.DisplayName, workspace.Name, workspace.URLKey, prefix, time.Now().UTC().Format("Monday, January 2, 2006"))
	fmt.Fprintf(&prompt, "Find and change things with the ricrac tools. They act as %s, so you can do what they can, except delete. "+
		"You can't run code, read files or browse the web.\n\n", user.Name)
	prompt.WriteString("- Look things up before you answer; never guess identifiers, names or counts.\n" +
		"- Link what you mention with the url the tools answer, like [" + prefix + "-12](" + self.cfg.PublicURL + "/" + workspace.URLKey + "/issue/" + prefix + "-12).\n" +
		"- Before changing more than 10 things in one go, list what you'd change and ask first.\n" +
		"- Be brief. Write Markdown.\n")
	guide, err := self.guide(ctx, actor)
	if err != nil {
		return "", err
	}
	if guide != "" {
		prompt.WriteString("\nThe workspace's notes for you, from its " + guideDocument + " document:\n\n" + guide + "\n")
	}
	return prompt.String(), nil
}

// guide is the workspace's Assistant document, empty without one.
func (self *AssistantService) guide(ctx context.Context, actor *auth.Actor) (string, error) {
	document, err := self.documents.Get(ctx, actor, &document_service.IDInput{ID: guideDocument})
	if err != nil && slices.Contains([]int{http.StatusBadRequest, http.StatusNotFound}, errdefs.As(err).Status) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(document.Content.Value)
	runes := []rune(content)
	if len(runes) > guideMaxLength {
		return string(runes[:guideMaxLength]), nil
	}
	return content, nil
}

// workDirs are the CLI's home and the empty directory it runs in.
func workDirs() (home, work string, err error) {
	base := filepath.Join(os.TempDir(), "ricrac-assistant")
	home, work = filepath.Join(base, "home"), filepath.Join(base, "work")
	for _, dir := range []string{home, work} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", "", err
		}
	}
	return home, work, nil
}
