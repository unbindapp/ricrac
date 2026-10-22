// Package seed_service is what new workspaces and teams start with, and the checks of their keys.
package seed_service

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"ricrac/internal/common/errdefs"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i SeedServiceInterface -p seed_service -s SeedService -o seed_service_iface.go
type SeedService struct {
	repo repositories.RepositoriesInterface
}

func NewSeedService(repo repositories.RepositoriesInterface) *SeedService {
	return &SeedService{repo: repo}
}

// WorkspaceDefaults adds project statuses, a few labels and customer statuses.
func (self *SeedService) WorkspaceDefaults(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID) error {
	return self.repo.Seed().WorkspaceDefaults(ctx, tx, workspace)
}

// CreateTeam creates a team with the default workflow and the creator as its first member.
func (self *SeedService) CreateTeam(ctx context.Context, tx repository.TxInterface, workspace, creator uuid.UUID, name, key string) (uuid.UUID, error) {
	return self.repo.Seed().CreateTeam(ctx, tx, workspace, creator, name, key)
}

// EnsureTriageState adds the Triage status a team needs once triage is turned on.
func (self *SeedService) EnsureTriageState(ctx context.Context, tx repository.TxInterface, team uuid.UUID) (uuid.UUID, error) {
	return self.repo.Seed().EnsureTriageState(ctx, tx, team)
}

// TeamKey is a team key from a name: its initials, or its first letters, up to five, uppercase.
func TeamKey(name string) string {
	var words []string
	for _, w := range strings.Fields(name) {
		if strings.IndexFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			words = append(words, w)
		}
	}
	isASCIIAlnum := func(r rune) bool { return r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) }
	var key []rune
	if len(words) > 1 {
		for _, w := range words {
			if i := strings.IndexFunc(w, isASCIIAlnum); i >= 0 && len(key) < 5 {
				key = append(key, rune(w[i]))
			}
		}
	} else {
		for _, r := range name {
			if isASCIIAlnum(r) && len(key) < 3 {
				key = append(key, r)
			}
		}
	}
	if len(key) == 0 {
		return "TEAM"
	}
	return strings.ToUpper(string(key))
}

// WorkspaceTeamKey is the key of a workspace's team from its URL key: acme-corp is AC, unbind is
// UNB. Leading digits are dropped, since a key starts with a letter.
func WorkspaceTeamKey(urlKey string) string {
	key := strings.TrimLeft(TeamKey(strings.ReplaceAll(urlKey, "-", " ")), "0123456789")
	if key == "" {
		return "TEAM"
	}
	return key
}

// CheckURLKey checks a workspace URL key: 2 to 32 lowercase letters, digits and dashes, not a
// path the server uses.
func CheckURLKey(key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	valid := len(key) >= 2 && len(key) <= 32 && !strings.HasPrefix(key, "-") && !strings.HasSuffix(key, "-")
	for _, b := range []byte(key) {
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-') {
			valid = false
		}
	}
	if !valid {
		return "", errdefs.BadRequest("The URL key is 2 to 32 lowercase letters, digits and dashes")
	}
	reserved := []string{"api", "v1", "mcp", "oauth", "assets", "files", "settings", "login", "admin", "docs", "llms.txt",
		"uploads", "privacy", "terms"}
	if slices.Contains(reserved, key) {
		return "", errdefs.BadRequest("%q is reserved", key)
	}
	return key, nil
}

// CheckTeamKey checks a team key: 1 to 7 letters and digits, starting with a letter.
func CheckTeamKey(key string) (string, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	valid := len(key) >= 1 && len(key) <= 7 && key[0] >= 'A' && key[0] <= 'Z'
	for _, b := range []byte(key) {
		if !(b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			valid = false
		}
	}
	if !valid {
		return "", errdefs.BadRequest("A team key is 1 to 7 letters and digits, starting with a letter, like ENG")
	}
	return key, nil
}
