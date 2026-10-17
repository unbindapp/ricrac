package mail

import (
	"bytes"
	"embed"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
	"unicode/utf16"
)

//go:embed templates/*
var templateFS embed.FS

const logoName = "logo.png"

var layoutFuncs = htmltemplate.FuncMap{
	"avatarColor":     AvatarColor,
	"msoStyle":        constHTML(`<!--[if mso]><style>table,td,a,span{font-family:'Segoe UI',Arial,sans-serif !important;}</style><![endif]-->`),
	"msoOpen":         constHTML(`<!--[if mso]><table role="presentation" width="600" align="center" cellpadding="0" cellspacing="0" border="0"><tr><td><![endif]-->`),
	"msoClose":        constHTML(`<!--[if mso]></td></tr></table><![endif]-->`),
	"preheaderSpacer": constHTML(strings.Repeat("&zwnj;&nbsp;", 90)),
}

func constHTML(markup string) func() htmltemplate.HTML {
	return func() htmltemplate.HTML { return htmltemplate.HTML(markup) }
}

var (
	htmlTemplate = htmltemplate.Must(htmltemplate.New("layout.html").Funcs(layoutFuncs).ParseFS(templateFS, "templates/layout.html"))
	textTemplate = texttemplate.Must(texttemplate.ParseFS(templateFS, "templates/layout.txt"))
	logoPNG      = mustRead("templates/" + logoName)
)

func mustRead(name string) []byte {
	data, err := templateFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return data
}

// Content is what an email says; every email has the same layout.
type Content struct {
	Subject   string   `json:"subject"`
	Preheader string   `json:"preheader,omitempty"`
	Heading   string   `json:"heading"`
	Intro     []string `json:"intro,omitempty"`
	Items     []Item   `json:"items,omitempty"`
	Button    Button   `json:"button"`
	Note      string   `json:"note,omitempty"`
	Footer    Footer   `json:"footer"`
}

// Item is one thing the email is about, like an issue, with lines of what happened to it.
type Item struct {
	Identifier string `json:"identifier,omitempty"`
	Title      string `json:"title"`
	URL        string `json:"url,omitempty"`
	Lines      []Line `json:"lines,omitempty"`
}

type Line struct {
	ActorID       string `json:"actorId,omitempty"`
	ActorName     string `json:"actorName,omitempty"`
	ActorInitials string `json:"actorInitials,omitempty"`
	Text          string `json:"text"`
}

type Button struct {
	Label string `json:"label,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Footer struct {
	InstanceName string `json:"instanceName,omitempty"`
	InstanceURL  string `json:"instanceUrl,omitempty"`
	ManageURL    string `json:"manageUrl,omitempty"`
	ManageLabel  string `json:"manageLabel,omitempty"`
}

type layout struct {
	Content
	LogoSrc htmltemplate.URL
}

func ActorLine(actorID, actorName, text string) Line {
	return Line{ActorID: actorID, ActorName: actorName, ActorInitials: Initials(actorName), Text: text}
}

// Render builds the text and HTML of an email to one person.
func Render(to string, content Content) (Message, error) {
	if content.Footer.InstanceName == "" {
		content.Footer.InstanceName = "Ricrac"
	}
	if content.Preheader == "" && len(content.Intro) > 0 {
		content.Preheader = content.Intro[0]
	}
	var text, html bytes.Buffer
	if err := textTemplate.Execute(&text, content); err != nil {
		return Message{}, err
	}
	if err := htmlTemplate.Execute(&html, layout{Content: content, LogoSrc: htmltemplate.URL("cid:" + logoName)}); err != nil {
		return Message{}, err
	}
	return Message{
		To:      to,
		Subject: content.Subject,
		Text:    text.String(),
		HTML:    html.String(),
		Inline:  []Inline{{Name: logoName, ContentType: "image/png", Data: logoPNG}},
	}, nil
}

var avatarPalette = []string{"#95a2b3", "#5e6ad2", "#26b5ce", "#0f783c", "#4cb782", "#f2c94c", "#f2994a", "#eb5757",
	"#db6e9e", "#bb87fc", "#8e6b4a", "#4b5563"}

// AvatarColor is the color of someone's initials avatar, the same as in the web app.
func AvatarColor(id string) string {
	var hash int32
	for _, unit := range utf16.Encode([]rune(id)) {
		hash = hash*31 + int32(unit)
	}
	magnitude := int64(hash)
	if magnitude < 0 {
		magnitude = -magnitude
	}
	return avatarPalette[magnitude%int64(len(avatarPalette))]
}

// Initials are the first letters of the first and last name, or the first two of a single name.
func Initials(name string) string {
	words := strings.Fields(name)
	first := func(word string, count int) string {
		runes := []rune(word)
		return string(runes[:min(count, len(runes))])
	}
	switch len(words) {
	case 0:
		return "?"
	case 1:
		return strings.ToUpper(first(words[0], 2))
	}
	return strings.ToUpper(first(words[0], 1) + first(words[len(words)-1], 1))
}
