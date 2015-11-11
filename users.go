package fs

import (
	"html/template"

	"github.com/google/go-github/github"
	"sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/apps/issues/issues"
)

func sgUser(user *sourcegraph.User) issues.User {
	return issues.User{
		Login:     user.Login,
		AvatarURL: avatarURL(user.Login),                            //template.URL(user.AvatarURL),
		HTMLURL:   template.URL("https://github.com/" + user.Login), // TODO.
	}
}

var (
	gh        = github.NewClient(nil)
	ghAvatars = make(map[string]template.URL)
)

// TODO.
func avatarURL(login string) template.URL {
	if avatarURL, ok := ghAvatars[login]; ok {
		return avatarURL
	}

	user, _, err := gh.Users.Get(login)
	if err != nil || user.AvatarURL == nil {
		return ""
	}
	ghAvatars[login] = template.URL(*user.AvatarURL + "&s=96")
	return ghAvatars[login]
}
