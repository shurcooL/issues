package fs

import (
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/shurcooL/issues"
	"github.com/shurcooL/notifications"
	"github.com/shurcooL/users"
)

// threadType is the notifications thread type for this service.
const threadType = "issues"

// ThreadType returns the notifications thread type for this service.
func (service) ThreadType() string { return threadType }

// subscribe subscribes user and anyone mentioned in body to the issue.
func (s service) subscribe(ctx context.Context, repo issues.RepoSpec, issueID uint64, user users.UserSpec, body string) error {
	if s.notifications == nil {
		return nil
	}

	subscribers := []users.UserSpec{user}

	// TODO: Find mentioned users in body.
	/*mentions, err := mentions(ctx, body)
	if err != nil {
		return err
	}
	subscribers = append(subscribers, mentions...)*/

	return s.notifications.Subscribe(ctx, threadType, notifications.RepoSpec{URI: repo.URI}, issueID, subscribers)
}

// markRead marks the specified issue as read for current user.
func (s service) markRead(ctx context.Context, repo issues.RepoSpec, issueID uint64) error {
	if s.notifications == nil {
		return nil
	}

	return s.notifications.MarkRead(ctx, threadType, notifications.RepoSpec{URI: repo.URI}, issueID)
}

// notify notifies all subscribed users of an update that shows up in their Notification Center.
func (s service) notify(ctx context.Context, repo issues.RepoSpec, issueID uint64, fragment string, actor users.UserSpec, createdAt time.Time) error {
	if s.notifications == nil {
		return nil
	}

	// TODO, THINK: Is this the best place/time?
	// Get issue from storage for to populate notification fields.
	fs := s.namespace(repo.URI)
	var issue issue
	err := jsonDecodeFile(fs, issueCommentPath(issueID, 0), &issue)
	if err != nil {
		return err
	}

	// THINK: Where should the logic to come up with the URL live? It's kinda related to the router/URL scheme of issuesapp...
	htmlURL := template.URL(fmt.Sprintf("https://%s/%v", repo.URI, issueID))
	if fragment != "" {
		htmlURL += template.URL("#" + fragment)
	}

	nr := notifications.NotificationRequest{
		Title:     issue.Title,
		Icon:      notificationIcon(issue.State),
		Color:     notificationColor(issue.State),
		Actor:     actor,
		UpdatedAt: createdAt,
		HTMLURL:   htmlURL,
	}

	return s.notifications.Notify(ctx, threadType, notifications.RepoSpec{URI: repo.URI}, issueID, nr)
}

// TODO: This is display/presentation logic; try to factor it out of the backend service implementation.
//       (Have it be provided to the service, maybe? Or another way.)
func notificationIcon(state issues.State) notifications.OcticonID {
	switch state {
	case issues.OpenState:
		return "issue-opened"
	case issues.ClosedState:
		return "issue-closed"
	default:
		return ""
	}
}

/* TODO
func (e event) Octicon() string {
	switch e.Event.Type {
	case issues.Reopened:
		return "octicon-primitive-dot"
	case issues.Closed:
		return "octicon-circle-slash"
	default:
		return "octicon-primitive-dot"
	}
}*/

func notificationColor(state issues.State) notifications.RGB {
	switch state {
	case issues.OpenState: // Open.
		return notifications.RGB{R: 0x6c, G: 0xc6, B: 0x44}
	case issues.ClosedState: // Closed.
		return notifications.RGB{R: 0xbd, G: 0x2c, B: 0x00}
	default:
		return notifications.RGB{}
	}
}
