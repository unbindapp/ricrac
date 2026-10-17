package notifications_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	notifications := srv.NotificationService
	oapi.Register(reg, oapi.Operation{
		Name: "get_notifications", Access: auth.Read, Routes: []string{"GET /notifications"},
		Description: "Your inbox, newest first: assignments, mentions, comments, replies, status changes and more, with how many " +
			"are unread. Archived notifications and snoozed ones (until they wake) are left out unless asked for.",
	}, notifications.List)
	oapi.Register(reg, oapi.Operation{
		Name: "mark_notification", Access: auth.Write, Routes: []string{"POST /notifications/mark"},
		Description: "Mark one or more of your notifications read or unread, archive or unarchive them, or snooze them until a " +
			"time (it comes back unread then) or unsnooze them.",
	}, notifications.Mark)
	oapi.Register(reg, oapi.Operation{
		Name: "mark_all_notifications_read", Access: auth.Write, Routes: []string{"POST /notifications/read-all"},
		Description: "Mark all your unread notifications read, optionally only those before a time or about one issue.",
	}, notifications.MarkAllRead)
}
