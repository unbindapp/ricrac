package utils

import (
	"strings"
	"time"

	"ricrac/internal/common/errdefs"
)

// IsNoneWord is whether a value means nothing, for project and initiative fields.
func IsNoneWord(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "null", "none":
		return true
	}
	return false
}

func CheckHealth(value string) (string, error) {
	for _, health := range []string{"onTrack", "atRisk", "offTrack"} {
		if strings.EqualFold(health, strings.TrimSpace(value)) {
			return health, nil
		}
	}
	return "", errdefs.BadRequest("health is onTrack, atRisk or offTrack, not \"%s\"", value)
}

// CheckResolution is a date resolution, nil when null or a null word.
func CheckResolution(value *string) (*string, error) {
	if value == nil || IsNoneWord(*value) {
		return nil, nil
	}
	for _, resolution := range []string{"month", "quarter", "halfYear", "year"} {
		if strings.EqualFold(resolution, strings.TrimSpace(*value)) {
			return &resolution, nil
		}
	}
	return nil, errdefs.BadRequest("A date resolution is month, quarter, halfYear or year, not \"%s\"", *value)
}

// OptionalDate is a date, nil when null or a null word.
func OptionalDate(value *string) (*time.Time, error) {
	if value == nil || IsNoneWord(*value) {
		return nil, nil
	}
	date, err := ParseDate(*value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

// EntityURL is a project's or initiative's page: kind is project or initiative.
func EntityURL(publicURL, workspaceKey, kind, name, slugID string) string {
	slug := slugID
	if words := NameSlug(name); words != "" {
		slug = words + "-" + slugID
	}
	return publicURL + "/" + workspaceKey + "/" + kind + "/" + slug
}
