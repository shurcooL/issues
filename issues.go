package issues

import (
	"fmt"
	"html/template"
	"time"

	"golang.org/x/net/context"
)

type RepoSpec struct {
	URI string // URI is clean '/'-separated URI. E.g, "user/repo".
}

type Service interface {
	List(ctx context.Context, repo RepoSpec, opt IssueListOptions) ([]Issue, error)
	Count(ctx context.Context, repo RepoSpec, opt IssueListOptions) (uint64, error)

	Get(ctx context.Context, repo RepoSpec, id uint64) (Issue, error)

	ListComments(ctx context.Context, repo RepoSpec, id uint64, opt interface{}) ([]Comment, error)
	ListEvents(ctx context.Context, repo RepoSpec, id uint64, opt interface{}) ([]Event, error)

	CreateComment(ctx context.Context, repo RepoSpec, id uint64, comment Comment) (Comment, error)

	Create(ctx context.Context, repo RepoSpec, issue Issue) (Issue, error)

	Edit(ctx context.Context, repo RepoSpec, id uint64, ir IssueRequest) (Issue, error)

	// TODO: This doesn't belong here, does it?
	CurrentUser(ctx context.Context) (User, error)
}

// Issue represents an issue on a repository.
type Issue struct {
	ID    uint64
	State State
	Title string
	Comment
	Replies int
}

// Comment represents a comment left on an issue.
type Comment struct {
	User      User
	CreatedAt time.Time
	Body      string
}

// User represents a user.
type User struct {
	Login     string
	AvatarURL template.URL
	HTMLURL   template.URL
}

// IssueRequest is a request to edit an issue.
type IssueRequest struct {
	State *State
	Title *string
}

// State represents the issue state.
type State string

const (
	OpenState   State = "open"
	ClosedState State = "closed"
)

func (ir IssueRequest) Validate() error {
	if ir.State != nil {
		switch *ir.State {
		case OpenState, ClosedState:
		default:
			return fmt.Errorf("bad state")
		}
	}
	if ir.Title != nil {
		// TODO.
	}
	return nil
}
