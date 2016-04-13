package fs

import (
	"fmt"

	"github.com/shurcooL/users"
	"golang.org/x/net/context"
)

func (s service) user(ctx context.Context, user users.UserSpec) users.User {
	u, err := s.users.Get(ctx, users.UserSpec{ID: user.ID, Domain: user.Domain})
	if err != nil {
		return users.User{
			UserSpec:  user,
			Login:     fmt.Sprintf("Anonymous %v", user.ID),
			AvatarURL: "https://secure.gravatar.com/avatar?d=mm&f=y&s=96",
			HTMLURL:   "",
		}
	}
	return users.User{
		UserSpec:  users.UserSpec{ID: u.ID, Domain: u.Domain},
		Login:     u.Login,
		AvatarURL: u.AvatarURL,
		HTMLURL:   u.HTMLURL,
	}
}
