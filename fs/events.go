package fs

import (
	"context"
	"time"

	"dmitri.shuralyov.com/state"
	eventpkg "github.com/shurcooL/events/event"
	"github.com/shurcooL/issues"
	"github.com/shurcooL/users"
)

func (s *service) logIssue(ctx context.Context, repo issues.RepoSpec, issueID uint64, fragment string, issue issue, actor users.User, action string, time time.Time) error {
	if s.events == nil {
		return nil
	}

	event := eventpkg.Event{
		Time:      time,
		Actor:     actor,
		Container: repo.URI,

		Payload: eventpkg.Issue{
			Action:       action,
			IssueTitle:   issue.Title,
			IssueBody:    issue.Body,
			IssueHTMLURL: htmlURL(repo.URI, issueID, fragment),
		},
	}
	return s.events.Log(ctx, event)
}

func (s *service) logIssueComment(ctx context.Context, repo issues.RepoSpec, issueID uint64, fragment string, actor users.User, time time.Time, body string) error {
	if s.events == nil {
		return nil
	}

	// TODO, THINK: Is this the best place/time? It's also being done in s.notify...
	// Get issue from storage for to populate event fields.
	var issue issue
	err := jsonDecodeFile(ctx, s.fs, issueCommentPath(repo, issueID, 0), &issue)
	if err != nil {
		return err
	}

	event := eventpkg.Event{
		Time:      time,
		Actor:     actor,
		Container: repo.URI,

		Payload: eventpkg.IssueComment{
			IssueTitle:     issue.Title,
			IssueState:     state.Issue(issue.State), // TODO: Make the conversion go away (by making issues.State type state.Issue).
			CommentBody:    body,
			CommentHTMLURL: htmlURL(repo.URI, issueID, fragment),
		},
	}
	return s.events.Log(ctx, event)
}
