package fs

import (
	"fmt"

	"github.com/shurcooL/users"
	"golang.org/x/net/context"
)

func (s service) user(ctx context.Context, user users.UserSpec) users.User {
	u, err := s.users.Get(ctx, user)
	if err != nil {
		return users.User{
			UserSpec:  user,
			Login:     fmt.Sprintf("Anonymous %v", user.ID),
			AvatarURL: "https://secure.gravatar.com/avatar?d=mm&f=y&s=96",
			HTMLURL:   "",
		}
	}
	return u
}
