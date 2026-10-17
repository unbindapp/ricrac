package releases_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	releases := srv.ReleaseService
	oapi.Register(reg, oapi.Operation{
		Name: "list_release_pipelines", Access: auth.Read, Routes: []string{"GET /release-pipelines"},
		Description: "List release pipelines by name: type (continuous: every merge ships; scheduled: releases are planned " +
			"ahead), whether they deploy to production and how many releases they have. Set includeStages / " +
			"includeTeams to also get their stages and teams.",
	}, releases.ListPipelines)
	oapi.Register(reg, oapi.Operation{
		Name: "get_release_pipeline", Access: auth.Read, Routes: []string{"GET /release-pipelines/{id}"},
		Description: "Read a release pipeline by id, slug or name, with its stages (in order) and teams.",
	}, releases.GetPipeline)
	oapi.Register(reg, oapi.Operation{
		Name: "save_release_pipeline", Access: auth.Write, Routes: []string{"POST /release-pipelines", "PATCH /release-pipelines/{id}"},
		Description: "Create a release pipeline (omit id; name is required) or update one (pass id, slug or name). New " +
			"pipelines get the stages Planned, In Progress, Released and Canceled; change them with " +
			"save_release_stage. Teams are given as key, name or id.",
	}, releases.SavePipeline)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_release_pipeline", Access: auth.Write, Routes: []string{"DELETE /release-pipelines/{id}"},
		Description: "Archive a release pipeline. Its releases stay readable.",
	}, releases.DeletePipeline)
	oapi.Register(reg, oapi.Operation{
		Name: "save_release_stage", Access: auth.Write, Routes: []string{"POST /release-stages", "PATCH /release-stages/{id}"},
		Description: "Add a stage to a release pipeline (omit id; pipeline, name and type are required) or change one (pass its " +
			"id). The type (planned, started, completed, canceled) decides which timestamps releases get when they " +
			"move into it.",
	}, releases.SaveStage)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_release_stage", Access: auth.Write, Routes: []string{"DELETE /release-stages/{id}"},
		Description: "Delete a release stage. When releases are in it, moveReleasesTo (another stage of the pipeline: name, " +
			"type or id) is required.",
	}, releases.DeleteStage)
	oapi.Register(reg, oapi.Operation{
		Name: "list_releases", Access: auth.Read, Routes: []string{"GET /releases"},
		Description: "List releases, newest first: name, version, pipeline, stage, dates, commit and how many issues they ship. " +
			"Narrow by pipeline, stage (a type like started works across pipelines; a stage name needs pipeline) or a " +
			"query on the name, version or commit. Archived releases are left out unless includeArchived is set.",
	}, releases.ListReleases)
	oapi.Register(reg, oapi.Operation{
		Name: "get_release", Access: auth.Read, Routes: []string{"GET /releases/{id}"},
		Description: "Read a release by id, slug or name, with the issues in it (with their status) and the release notes " +
			"that cover it.",
	}, releases.GetRelease)
	oapi.Register(reg, oapi.Operation{
		Name: "save_release", Access: auth.Write, Routes: []string{"POST /releases", "PATCH /releases/{id}"},
		Description: "Create a release (omit id; name and pipeline are required) or update one (pass id, slug or name). " +
			"Omitted fields stay as they are; null clears a field. New releases start in the pipeline's first planned " +
			"stage unless stage is given. Moving to a started, completed or canceled stage stamps startedAt, " +
			"completedAt or canceledAt unless you pass them; moving back to planned clears them. Add issues to a " +
			"release with save_issue (addReleases, removeReleases, setReleases).",
	}, releases.SaveRelease)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_release", Access: auth.Write, Routes: []string{"DELETE /releases/{id}"},
		Description: "Archive a release. It stays readable and keeps its issues.",
	}, releases.DeleteRelease)
	oapi.Register(reg, oapi.Operation{
		Name: "list_release_notes", Access: auth.Read, Routes: []string{"GET /release-notes"},
		Description: "List release notes, newest first, optionally of one pipeline: title, pipeline, the releases they cover " +
			"and who wrote them. Read the content with get_release_note.",
	}, releases.ListNotes)
	oapi.Register(reg, oapi.Operation{
		Name: "get_release_note", Access: auth.Read, Routes: []string{"GET /release-notes/{id}"},
		Description: "Read a release note by id or slug: title, Markdown content, pipeline and the releases it covers.",
	}, releases.GetNote)
	oapi.Register(reg, oapi.Operation{
		Name: "save_release_note", Access: auth.Write, Routes: []string{"POST /release-notes", "PATCH /release-notes/{id}"},
		Description: "Create release notes (omit id; pipeline is required) or update them (pass id or slug). Name the releases " +
			"they cover with releases, or with rangeFromRelease and rangeToRelease for every release of the pipeline " +
			"created between the two, inclusive. The title defaults to the newest release's name and version. Use " +
			"patch to edit part of the content without resending it.",
	}, releases.SaveNote)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_release_note", Access: auth.Write, Routes: []string{"DELETE /release-notes/{id}"},
		Description: "Delete release notes. The releases stay.",
	}, releases.DeleteNote)
}
