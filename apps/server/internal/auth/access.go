// Package auth is who may do what: roles, token scopes, signed values, sessions and secrets.
package auth

import "ricrac/internal/common/errdefs"

type Role int

const (
	Guest Role = iota
	Member
	Admin
	Owner
)

func ParseRole(value string) (Role, bool) {
	switch value {
	case "guest":
		return Guest, true
	case "member":
		return Member, true
	case "admin":
		return Admin, true
	case "owner":
		return Owner, true
	}
	return Guest, false
}

func (r Role) String() string {
	return [...]string{"guest", "member", "admin", "owner"}[r]
}

// Access is what a token may do. Sessions and keys without scopes may do everything their role
// allows.
type Access int

const (
	Read Access = iota
	Write
	AdminAccess
)

func (a Access) String() string {
	return [...]string{"read", "write", "admin"}[a]
}

var AllScopes = []string{"read", "write", "admin"}

var EveryAccess = []Access{Read, Write, AdminAccess}

func ParseScopes(scopes []string) []Access {
	var found []Access
	for _, scope := range scopes {
		switch scope {
		case "read":
			found = append(found, Read)
		case "write":
			found = append(found, Write)
		case "admin":
			found = append(found, AdminAccess)
		}
	}
	return found
}

func ScopeError(access Access) *errdefs.Error {
	return errdefs.Forbidden("This token lacks the %q scope", access.String())
}
