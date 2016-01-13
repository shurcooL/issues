package fs

import (
	"fmt"

	"golang.org/x/net/context"
	"src.sourcegraph.com/apps/tracker/issues"
)

func sgUser(_ context.Context, user issues.UserSpec) issues.User {
	return issues.User{
		UserSpec:  user,
		Login:     fmt.Sprintf("Anonymous %v", user.ID),
		AvatarURL: "https://secure.gravatar.com/avatar?d=mm&f=y&s=96",
		HTMLURL:   "",
	}
}
