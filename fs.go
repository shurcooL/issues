// Package fs implements issues.Service using a filesystem.
package fs

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/go-github/github"

	"golang.org/x/net/context"
	"sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/platform/apps/issues2/issues"
)

// NewService ...
func NewService() issues.Service {
	return service{
		// TODO.
		dir: "/Users/Dmitri/.sourcegraph/issues/user/Go-Package-Store.next",
	}
}

type service struct {
	dir string

	// TODO.
	issues.Service
}

func (s service) ListByRepo(ctx context.Context, repo issues.RepoSpec, opt interface{}) ([]issues.Issue, error) {
	sg := sourcegraph.NewClientFromContext(ctx)

	var is []issues.Issue

	dirs, err := ioutil.ReadDir(s.dir)
	if err != nil {
		return is, err
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		id, err := parseUint64(dir.Name())
		if err != nil {
			continue
		}

		var issue issue
		err = jsonDecodeFile(filepath.Join(s.dir, dir.Name(), "0.json"), &issue)
		if err != nil {
			return is, err
		}

		user, err := sg.Users.Get(ctx, &sourcegraph.UserSpec{UID: issue.AuthorUID})
		if err != nil {
			return is, err
		}

		is = append(is, issues.Issue{
			ID:    id,
			State: issue.State,
			Title: issue.Title,
			Comment: issues.Comment{
				User: issues.User{
					Login:     user.Login,
					AvatarURL: avatarURL(user.Login),                            //template.URL(user.AvatarURL),
					HTMLURL:   template.URL("https://github.com/" + user.Login), // TODO.
				},
				CreatedAt: issue.CreatedAt,
			},
		})
	}

	return is, nil
}

func (s service) Get(ctx context.Context, repo issues.RepoSpec, id uint64) (issues.Issue, error) {
	sg := sourcegraph.NewClientFromContext(ctx)

	var issue issue
	err := jsonDecodeFile(filepath.Join(s.dir, formatUint64(id), "0.json"), &issue)
	if err != nil {
		return issues.Issue{}, err
	}

	user, err := sg.Users.Get(ctx, &sourcegraph.UserSpec{UID: issue.AuthorUID})
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    id,
		State: issue.State,
		Title: issue.Title,
		Comment: issues.Comment{
			User: issues.User{
				Login:     user.Login,
				AvatarURL: avatarURL(user.Login),                            //template.URL(user.AvatarURL),
				HTMLURL:   template.URL("https://github.com/" + user.Login), // TODO.
			},
			CreatedAt: issue.CreatedAt,
		},
	}, nil
}

func (s service) ListComments(ctx context.Context, repo issues.RepoSpec, id uint64, opt interface{}) ([]issues.Comment, error) {
	sg := sourcegraph.NewClientFromContext(ctx)

	var comments []issues.Comment

	dir := filepath.Join(s.dir, formatUint64(id))
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return comments, err
	}

	for _, fi := range fis {
		var comment comment
		err = jsonDecodeFile(filepath.Join(dir, fi.Name()), &comment)
		if err != nil {
			return comments, err
		}

		user, err := sg.Users.Get(ctx, &sourcegraph.UserSpec{UID: comment.AuthorUID})
		if err != nil {
			return comments, err
		}

		comments = append(comments, issues.Comment{
			User: issues.User{
				Login:     user.Login,
				AvatarURL: avatarURL(user.Login),                            //template.URL(user.AvatarURL),
				HTMLURL:   template.URL("https://github.com/" + user.Login), // TODO.
			},
			CreatedAt: comment.CreatedAt,
			Body:      comment.Body,
		})
	}

	return comments, nil
}

// TODO.
func (service) CurrentUser() issues.User {
	return issues.User{
		Login:     "shurcooL",
		AvatarURL: "https://avatars.githubusercontent.com/u/1924134?v=3&s=96",
		HTMLURL:   "https://github.com/shurcooL",
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

func jsonDecodeFile(path string, v interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = json.NewDecoder(f).Decode(v)
	_ = f.Close()
	if err != nil {
		return err
	}
	return nil
}

func formatUint64(n uint64) string         { return strconv.FormatUint(n, 10) }
func parseUint64(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) }
