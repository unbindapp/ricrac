package assistant_service

type AssistantSettingsInput struct {
	Enabled *bool    `json:"enabled"`
	UserIDs []string `json:"userIds" doc:"Everyone who may ask Claude, members but not guests; leave out to keep them."`
}

type AskInput struct {
	Prompt    string  `json:"prompt"`
	SessionID *string `json:"sessionId" doc:"The conversation to continue; leave out to start one."`
	Page      *string `json:"page" doc:"The path of the page open in the web app, like /acme/issue/ENG-12."`
}
