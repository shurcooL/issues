package issues

import (
	"time"

	"github.com/shurcooL/users"
)

// Event represents an event that occurred around an issue.
type Event struct {
	ID        uint64
	Actor     users.User
	CreatedAt time.Time
	Type      EventType
	Rename    *Rename // Rename is only provided for Renamed events.
	Label     *Label  // Label is only provided for Labeled and Unlabeled events.
}

// EventType is the type of an event.
type EventType string

const (
	// Reopened is when an issue is reopened.
	Reopened EventType = "reopened"
	// Closed is when an issue is closed.
	Closed EventType = "closed"
	// Renamed is when an issue is renamed.
	Renamed EventType = "renamed"
	// Labeled is when an issue is labeled.
	Labeled EventType = "labeled"
	// Unlabeled is when an issue is unlabeled.
	Unlabeled EventType = "unlabeled"
	// CommentDeleted is when an issue comment is deleted.
	CommentDeleted EventType = "comment_deleted"
)

// Valid returns non-nil error if the event type is invalid.
func (et EventType) Valid() bool {
	switch et {
	case Reopened, Closed, Renamed, Labeled, Unlabeled, CommentDeleted:
		return true
	default:
		return false
	}
}

// Rename provides details for a Renamed event.
type Rename struct {
	From string
	To   string
}
