package fs

import (
	"path"
	"time"

	"github.com/shurcooL/issues"
	"github.com/shurcooL/users"
)

// userSpec is an on-disk representation of a specification for a user.
type userSpec struct {
	ID     uint64
	Domain string `json:",omitempty"`
}

func fromUserSpec(us users.UserSpec) userSpec {
	return userSpec{ID: us.ID, Domain: us.Domain}
}

func (us userSpec) UserSpec() users.UserSpec {
	return users.UserSpec{ID: us.ID, Domain: us.Domain}
}

func (us userSpec) Equal(other users.UserSpec) bool {
	return us.Domain == other.Domain && us.ID == other.ID
}

// issue is an on-disk representation of an issue.
type issue struct {
	State issues.State
	Title string
	comment
}

// comment is an on-disk representation of a comment.
type comment struct {
	Author    userSpec
	CreatedAt time.Time
	Body      string
	Reactions []reaction `json:",omitempty"`
}

// reaction is an on-disk representation of a reaction.
type reaction struct {
	EmojiID issues.EmojiID
	Authors []userSpec // Order does not matter; this would be better represented as a set like map[userSpec]struct{}, but we're using JSON and it doesn't support that.
}

// event is an on-disk representation of an event.
type event struct {
	Actor     userSpec
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
