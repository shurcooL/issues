package issues

import "time"

type IssuesService interface {
	Comment() Comment
}

// Comment represents a comment left on an issue.
type Comment struct {
	ID        int
	Body      string
	User      User
	CreatedAt time.Time
}

// User represents a user.
type User struct {
	Login     string
	AvatarURL string
	Name      string
}
