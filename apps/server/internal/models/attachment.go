package models

import (
	"encoding/json"

	"ricrac/ent"
	"ricrac/internal/common/utils"
)

// AttachmentOf needs the attachment's creator and issue (with its team and state) loaded.
func AttachmentOf(a *ent.Attachment) Attachment {
	return Attachment{
		ID: ID(a.ID), URL: a.URL, Title: a.Title, Subtitle: a.Subtitle, SourceType: utils.OrEmpty(a.SourceType),
		Metadata: Object(a.Metadata), IconURL: a.IconURL, Creator: UserRefOf(a.Edges.Creator), Issue: IssueRefOf(a.Edges.Issue),
		CreatedAt: Time(a.CreatedAt), UpdatedAt: Time(a.UpdatedAt),
	}
}

// Object is a JSON object column, nil for SQL NULL.
func Object(raw json.RawMessage) M {
	if len(raw) == 0 {
		return nil
	}
	var object M
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil
	}
	return object
}
