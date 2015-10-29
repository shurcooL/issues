package issues

import (
	"html/template"
	"time"
)

type RepoSpec struct {
	Owner string
	Repo  string
}

type Service interface {
	ListByRepo(repo RepoSpec, opt interface{}) ([]Issue, error)

	Get(repo RepoSpec, id uint64) (Issue, error)

	ListComments(repo RepoSpec, id uint64, opt interface{}) ([]Comment, error)

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
	Body      string
	User      User
	CreatedAt time.Time
}

// User represents a user.
type User struct {
	Login     string
	AvatarURL template.URL
	HTMLURL   template.URL
}
