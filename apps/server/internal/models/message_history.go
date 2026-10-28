package models

import (
	"time"

	"ricrac/ent"
)

type MessageVersion struct {
	Body      string `json:"body"`
	WrittenAt string `json:"writtenAt" doc:"When it was written this way."`
	Current   bool   `json:"current" doc:"Whether this is how it reads now."`
}

type MessageHistory struct {
	Versions []MessageVersion `json:"versions" doc:"Oldest first, ending with the current body."`
}

// MessageHistoryOf is the earlier versions, oldest first, then the current body, written when it
// was last edited, or created if never.
func MessageHistoryOf(earlier []*ent.MessageVersion, body string, edited *time.Time, created time.Time) MessageHistory {
	versions := make([]MessageVersion, 0, len(earlier)+1)
	for _, v := range earlier {
		versions = append(versions, MessageVersion{Body: v.Body, WrittenAt: Time(v.WrittenAt)})
	}
	written := created
	if edited != nil {
		written = *edited
	}
	return MessageHistory{Versions: append(versions, MessageVersion{Body: body, WrittenAt: Time(written), Current: true})}
}
