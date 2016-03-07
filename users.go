package fs

import (
	"fmt"

	"github.com/shurcooL/users"
	"golang.org/x/net/context"
	"src.sourcegraph.com/apps/tracker/issues"
)

func (s service) issuesUser(ctx context.Context, user issues.UserSpec) issues.User {
	u, err := s.users.Get(ctx, users.UserSpec{ID: user.ID, Domain: user.Domain})
	if err != nil {
		return issues.User{
			UserSpec:  user,
			Login:     fmt.Sprintf("Anonymous %v", user.ID),
			AvatarURL: "https://secure.gravatar.com/avatar?d=mm&f=y&s=96",
			HTMLURL:   "",
		}
	}
	return issues.User{
		UserSpec:  issues.UserSpec{ID: u.ID, Domain: u.Domain},
		Login:     u.Login,
		AvatarURL: u.AvatarURL,
		HTMLURL:   u.HTMLURL,
	}
}
