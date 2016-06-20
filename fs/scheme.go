package fs

import (
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/shurcooL/issues"
	"github.com/shurcooL/reactions"
	"github.com/shurcooL/users"
	"golang.org/x/net/webdav"
)

// userSpec is an on-disk representation of users.UserSpec.
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

// issue is an on-disk representation of issues.Issue.
type issue struct {
	State issues.State
	Title string
	comment
}

// comment is an on-disk representation of issues.Comment.
type comment struct {
	Author    userSpec
	CreatedAt time.Time
	Body      string
	Reactions []reaction `json:",omitempty"`
}

// reaction is an on-disk representation of reactions.Reaction.
type reaction struct {
	EmojiID reactions.EmojiID
	Authors []userSpec // First entry is first person who reacted.
}

// event is an on-disk representation of issues.Event.
type event struct {
	Actor     userSpec
	CreatedAt time.Time
	Type      issues.EventType
	Rename    *issues.Rename `json:",omitempty"`
}

// Tree layout:
//
// 	root
// 	└── domain.com
// 	    └── path
// 	        └── issues
// 	            ├── 1
// 	            │   ├── 0 - encoded issue
// 	            │   ├── 1 - encoded comment
// 	            │   ├── 2
// 	            │   └── events
// 	            │       ├── 1 - encoded event
// 	            │       └── 2
// 	            └── 2
// 	                ├── 0
// 	                └── events

const (
	// issuesDir is '/'-separated path for issue storage.
	issuesDir = "issues"

	// eventsDir is dir name for issue events.
	eventsDir = "events"
)

// TODO: Merge this path segment into issuesDir, etc.
func (s service) namespace(repoURI string) webdav.FileSystem {
	return webdav.Dir(filepath.Join(s.root, filepath.FromSlash(repoURI)))
}
func (s service) createNamespace(repoURI string) error {
	// Only needed for first issue in the repo.
	// TODO: Make this better, use vfsutil.MkdirAll(fs webdav.FileSystem, ...).
	//       Consider implicit dir adapter?
	return os.MkdirAll(filepath.Join(s.root, filepath.FromSlash(repoURI), issuesDir), 0755)
}

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
