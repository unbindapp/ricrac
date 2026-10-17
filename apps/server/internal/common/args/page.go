package args

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"ricrac/internal/common/errdefs"
)

// Page is paging, built from a list operation's limit and cursor arguments.
type Page struct {
	Limit  *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor *string `json:"cursor" doc:"The endCursor of the previous page, to continue after it."`
}

// Size is how many to return.
func (p Page) Size() int64 {
	if p.Limit == nil {
		return 50
	}
	return min(max(*p.Limit, 1), 250)
}

// Offset is where the page starts.
func (p Page) Offset() (int64, error) {
	if p.Cursor == nil {
		return 0, nil
	}
	invalid := errdefs.BadRequest("Invalid cursor; pass the endCursor of the previous page")
	bytes, err := base64.RawURLEncoding.DecodeString(*p.Cursor)
	if err != nil {
		return 0, invalid
	}
	n, ok := strings.CutPrefix(string(bytes), "o:")
	if !ok {
		return 0, invalid
	}
	offset, err := strconv.ParseInt(n, 10, 64)
	if err != nil {
		return 0, invalid
	}
	return offset, nil
}

// Wrap takes the Size()+1 rows fetched from Offset(), trims the extra row and says whether
// there are more.
func (p Page) Wrap(nodes []any) (M, error) {
	if nodes == nil {
		nodes = []any{}
	}
	limit := int(p.Size())
	hasNext := len(nodes) > limit
	if hasNext {
		nodes = nodes[:limit]
	}
	offset, err := p.Offset()
	if err != nil {
		return nil, err
	}
	var endCursor any
	if hasNext {
		endCursor = base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "o:%d", offset+int64(len(nodes))))
	}
	return M{"nodes": nodes, "pageInfo": M{"hasNextPage": hasNext, "endCursor": endCursor}}, nil
}

type PageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor" doc:"Pass it as cursor to get the next page."`
}

// PageOf is the shape of a list operation's result.
type PageOf[T any] struct {
	Nodes    []T      `json:"nodes"`
	PageInfo PageInfo `json:"pageInfo"`
}

// TextEdit is one edit of a Markdown text, applied in order with the others of a patch.
type TextEdit struct {
	Op         string `json:"op"`
	OldString  string `json:"old_string,omitempty"`
	NewString  string `json:"new_string,omitempty"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
	Anchor     string `json:"anchor,omitempty"`
	Text       string `json:"text,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
}

func (TextEdit) JSONSchema() M {
	variant := func(op, description string, properties M, required ...string) M {
		properties["op"] = M{"type": "string", "const": op}
		list := []any{"op"}
		for _, r := range required {
			list = append(list, r)
		}
		return M{"type": "object", "description": description, "properties": properties, "required": list}
	}
	text := M{"type": "string"}
	return M{"oneOf": []any{
		variant("replace", "Replace text that occurs exactly once (or every occurrence with replace_all).",
			M{"old_string": text, "new_string": text, "replace_all": M{"type": "boolean", "default": false}}, "old_string", "new_string"),
		variant("insert_before", "Insert text directly before an anchor that occurs exactly once.",
			M{"anchor": text, "text": text}, "anchor", "text"),
		variant("insert_after", "Insert text directly after an anchor that occurs exactly once.",
			M{"anchor": text, "text": text}, "anchor", "text"),
		variant("prepend", "Insert text at the very start.", M{"text": text}, "text"),
		variant("append", "Insert text at the very end.", M{"text": text}, "text"),
		variant("replace_range", "Replace from `from` (inclusive) up to `to` (exclusive, kept), each occurring once.",
			M{"from": text, "to": text, "new_string": text}, "from", "to", "new_string"),
	}}
}

func findOnce(content, needle string) (int, error) {
	if needle == "" {
		return 0, errdefs.BadRequest("Patch anchors can't be empty")
	}
	at := strings.Index(content, needle)
	if at < 0 {
		return 0, errdefs.BadRequest("Patch text not found: %q", needle)
	}
	if strings.Contains(content[at+len(needle):], needle) {
		return 0, errdefs.BadRequest("Patch text occurs more than once, add context: %q", needle)
	}
	return at, nil
}

// ApplyPatch applies every edit or none.
func ApplyPatch(content string, edits []TextEdit) (string, error) {
	text := content
	for _, edit := range edits {
		switch edit.Op {
		case "replace":
			if edit.ReplaceAll {
				if edit.OldString == "" || !strings.Contains(text, edit.OldString) {
					return "", errdefs.BadRequest("Patch text not found: %q", edit.OldString)
				}
				text = strings.ReplaceAll(text, edit.OldString, edit.NewString)
				continue
			}
			at, err := findOnce(text, edit.OldString)
			if err != nil {
				return "", err
			}
			text = text[:at] + edit.NewString + text[at+len(edit.OldString):]
		case "insert_before":
			at, err := findOnce(text, edit.Anchor)
			if err != nil {
				return "", err
			}
			text = text[:at] + edit.Text + text[at:]
		case "insert_after":
			at, err := findOnce(text, edit.Anchor)
			if err != nil {
				return "", err
			}
			at += len(edit.Anchor)
			text = text[:at] + edit.Text + text[at:]
		case "prepend":
			text = edit.Text + text
		case "append":
			text += edit.Text
		case "replace_range":
			start, err := findOnce(text, edit.From)
			if err != nil {
				return "", err
			}
			end, err := findOnce(text[start:], edit.To)
			if err != nil {
				return "", err
			}
			text = text[:start] + edit.NewString + text[start+end:]
		default:
			return "", errdefs.BadRequest("Unknown patch op %q; use replace, insert_before, insert_after, prepend, append or replace_range", edit.Op)
		}
	}
	return text, nil
}

// NewContent is the new content from either a full replacement or a patch of the current one;
// nil when neither was given.
func NewContent(current string, replacement *string, patch []TextEdit) (*string, error) {
	switch {
	case replacement != nil && patch != nil:
		return nil, errdefs.BadRequest("Pass either the full content or a patch, not both")
	case replacement != nil:
		return replacement, nil
	case patch != nil:
		text, err := ApplyPatch(current, patch)
		if err != nil {
			return nil, err
		}
		return &text, nil
	}
	return nil, nil
}

// WrapOf is Wrap for typed nodes.
func WrapOf[T any](p Page, nodes []T) (PageOf[T], error) {
	if nodes == nil {
		nodes = []T{}
	}
	limit := int(p.Size())
	hasNext := len(nodes) > limit
	if hasNext {
		nodes = nodes[:limit]
	}
	offset, err := p.Offset()
	if err != nil {
		return PageOf[T]{}, err
	}
	var endCursor *string
	if hasNext {
		cursor := base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "o:%d", offset+int64(len(nodes))))
		endCursor = &cursor
	}
	return PageOf[T]{Nodes: nodes, PageInfo: PageInfo{HasNextPage: hasNext, EndCursor: endCursor}}, nil
}
