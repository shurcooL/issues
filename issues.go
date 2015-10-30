package issues

import (
	"html/template"
	"time"

	"golang.org/x/net/context"
)

type RepoSpec struct {
	Owner string
	Repo  string
}

type Service interface {
	ListByRepo(ctx context.Context, repo RepoSpec, opt interface{}) ([]Issue, error)

	Get(ctx context.Context, repo RepoSpec, id uint64) (Issue, error)

	ListComments(ctx context.Context, repo RepoSpec, id uint64, opt interface{}) ([]Comment, error)

	CreateComment(ctx context.Context, repo RepoSpec, id uint64, comment Comment) (Comment, error)

	Create(ctx context.Context, repo RepoSpec, issue Issue) (Issue, error)

	// TODO: Play things.
	Comment() Comment
	CurrentUser() User
}

// Issue represents an issue on a repository.
type Issue struct {
	ID    uint64
	State string
	Title string
	Comment
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
