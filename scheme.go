package fs

import (
	"path"
	"time"

	"src.sourcegraph.com/apps/tracker/issues"
)

/* TODO.
// userSpec is an on-disk representation of a specification for a user.
type userSpec struct {
	ID     uint64
	Domain string `json:",omitempty"`
}*/

// issue is an on-disk representation of an issue.
type issue struct {
	State issues.State
	Title string
	comment
}

// comment is an on-disk representation of a comment.
type comment struct {
	AuthorUID int32
	CreatedAt time.Time
	Body      string
	Reactions []reaction `json:",omitempty"`
}

// reaction is an on-disk representation of a reaction.
type reaction struct {
	EmojiID    issues.EmojiID
	AuthorUIDs []int32 // Order does not matter; this would be better represented as a set like map[int32]struct{}, but we're using JSON and it doesn't support that.
}

// event is an on-disk representation of an event.
type event struct {
	ActorUID  int32
	CreatedAt time.Time
	Type      issues.EventType
	Rename    *issues.Rename `json:",omitempty"`
}

const (
	// issuesDir is '/'-separated path for issue storage.
	issuesDir = "issues"

	// eventsDir is dir name for issue events.
	eventsDir = "events"
)

func issueDir(issueID uint64) string {
	return path.Join(issuesDir, formatUint64(issueID))
}

func issueCommentPath(issueID, commentID uint64) string {
	return path.Join(issuesDir, formatUint64(issueID), formatUint64(commentID))
}

func issueEventsDir(issueID uint64) string {
	return path.Join(issuesDir, formatUint64(issueID), eventsDir)
}

func issueEventPath(issueID, eventID uint64) string {
	return path.Join(issuesDir, formatUint64(issueID), eventsDir, formatUint64(eventID))
}
