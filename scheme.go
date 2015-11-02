package fs

import (
	"time"

	"src.sourcegraph.com/sourcegraph/platform/apps/issues2/issues"
)

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
}

// event is an on-disk representation of an event.
type event struct {
	ActorUID  int32
	CreatedAt time.Time
	Type      issues.EventType
	Rename    *issues.Rename
}
