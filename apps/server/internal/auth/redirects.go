package auth

import "strings"

// IsAllowedRedirect allows https anywhere, http only on loopback (native clients such as Claude
// Code), or an app's own URI scheme. PKCE or the client secret protects the code in every case.
func IsAllowedRedirect(uri string) bool {
	if len(uri) > 2000 || strings.Contains(uri, "#") {
		return false
	}
	scheme, rest, ok := strings.Cut(uri, "://")
	if !ok {
		return false
	}
	switch scheme {
	case "https":
		return rest != ""
	case "http":
		_, _, loopback := LoopbackParts(uri)
		return loopback
	case "javascript", "data", "file", "vbscript", "":
		return false
	}
	return strings.Trim(scheme, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+.-") == ""
}

// RedirectMatches is an exact match, except that loopback clients may use any port (RFC 8252
// section 7.3).
func RedirectMatches(registered, requested string) bool {
	if registered == requested {
		return true
	}
	host1, path1, ok1 := LoopbackParts(registered)
	host2, path2, ok2 := LoopbackParts(requested)
	return ok1 && ok2 && host1 == host2 && path1 == path2
}

// LoopbackParts is the host and path of an http URI on a loopback host.
func LoopbackParts(uri string) (string, string, bool) {
	rest, ok := strings.CutPrefix(uri, "http://")
	if !ok {
		return "", "", false
	}
	authority, path, _ := strings.Cut(rest, "/")
	host := StripPort(authority)
	if host != "localhost" && host != "127.0.0.1" && host != "[::1]" {
		return "", "", false
	}
	return host, path, true
}

// StripPort is the host of an authority, without a numeric port.
func StripPort(authority string) string {
	i := strings.LastIndex(authority, ":")
	if i < 0 {
		return authority
	}
	port := authority[i+1:]
	if port != "" && strings.Trim(port, "0123456789") == "" {
		return authority[:i]
	}
	return authority
}
