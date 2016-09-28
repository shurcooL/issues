package fs

import (
	"fmt"
	"path"
	"time"

	"github.com/shurcooL/issues"
	"github.com/shurcooL/reactions"
	"github.com/shurcooL/users"
	"github.com/shurcooL/webdavfs/vfsutil"
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

// rgb is an on-disk representation of issues.RGB.
type rgb struct {
	R, G, B uint8
}

func fromRGB(c issues.RGB) rgb {
	return rgb(c)
}

func (c rgb) RGB() issues.RGB {
	return issues.RGB(c)
}

// issue is an on-disk representation of issues.Issue.
type issue struct {
	State  issues.State
	Title  string
	Labels []label `json:",omitempty"`
	comment
}

// label is an on-disk representation of issues.Label.
type label struct {
	Name  string
	Color rgb
}

// comment is an on-disk representation of issues.Comment.
type comment struct {
	Author    userSpec
	CreatedAt time.Time
	Edited    *edited `json:",omitempty"`
	Body      string
	Reactions []reaction `json:",omitempty"`
}

type edited struct {
	By userSpec
	At time.Time
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
	Label     *label         `json:",omitempty"`
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

func (s service) createNamespace(repo issues.RepoSpec) error {
	if path.Clean("/"+repo.URI) != "/"+repo.URI {
		return fmt.Errorf("invalid repo.URI (not clean): %q", repo.URI)
	}

	// Only needed for first issue in the repo.
	// THINK: Consider implicit dir adapter?
	return vfsutil.MkdirAll(s.fs, issuesDir(repo), 0755)
}

// issuesDir is '/'-separated path to issue storage dir.
func issuesDir(repo issues.RepoSpec) string {
	return path.Join(repo.URI, "issues")
}

func issueDir(repo issues.RepoSpec, issueID uint64) string {
	return path.Join(repo.URI, "issues", formatUint64(issueID))
}

func issueCommentPath(repo issues.RepoSpec, issueID, commentID uint64) string {
	return path.Join(repo.URI, "issues", formatUint64(issueID), formatUint64(commentID))
}

// issueEventsDir is '/'-separated path to issue events dir.
func issueEventsDir(repo issues.RepoSpec, issueID uint64) string {
	return path.Join(repo.URI, "issues", formatUint64(issueID), "events")
}

func issueEventPath(repo issues.RepoSpec, issueID, eventID uint64) string {
	return path.Join(repo.URI, "issues", formatUint64(issueID), "events", formatUint64(eventID))
}
