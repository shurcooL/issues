// Package github implements issues.Service using GitHub API client.
package github

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/net/context"
	"src.sourcegraph.com/apps/issues/issues"
)

func NewService(client *github.Client) issues.Service {
	if client == nil {
		client = github.NewClient(nil)
	}
	return service{
		cl: client,
	}
}

type service struct {
	cl *github.Client
}

func (s service) List(_ context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) ([]issues.Issue, error) {
	repo := ghRepoSpec(rs)
	ghOpt := github.IssueListByRepoOptions{}
	switch opt.State {
	case issues.OpenState:
		// Do nothing, this is the GitHub default.
	case issues.ClosedState:
		ghOpt.State = "closed"
	}
	ghIssuesAndPRs, _, err := s.cl.Issues.ListByRepo(repo.Owner, repo.Repo, &ghOpt)
	if err != nil {
		return nil, err
	}

	var is []issues.Issue
	for _, issue := range ghIssuesAndPRs {
		// Filter out PRs.
		if issue.PullRequestLinks != nil {
			continue
		}

		is = append(is, issues.Issue{
			ID:    uint64(*issue.Number),
			State: issues.State(*issue.State),
			Title: *issue.Title,
			Comment: issues.Comment{
				User: issues.User{
					Login:     *issue.User.Login,
					AvatarURL: template.URL(*issue.User.AvatarURL),
					HTMLURL:   template.URL(*issue.User.HTMLURL),
				},
				CreatedAt: *issue.CreatedAt,
			},
			Replies: *issue.Comments,
		})
	}

	return is, nil
}

func (s service) Count(_ context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) (uint64, error) {
	repo := ghRepoSpec(rs)
	var count uint64

	// Count Issues and PRs (since there appears to be no way to count just issues in GitHub API).
	{
		ghOpt := github.IssueListByRepoOptions{ListOptions: github.ListOptions{PerPage: 1}}
		switch opt.State {
		case issues.OpenState:
			// Do nothing, this is the GitHub default.
		case issues.ClosedState:
			ghOpt.State = "closed"
		}
		ghIssuesAndPRs, ghIssuesAndPRsResp, err := s.cl.Issues.ListByRepo(repo.Owner, repo.Repo, &ghOpt)
		if err != nil {
			return 0, err
		}
		if ghIssuesAndPRsResp.LastPage != 0 {
			count = uint64(ghIssuesAndPRsResp.LastPage)
		} else {
			count = uint64(len(ghIssuesAndPRs))
		}
	}

	// Subtract PRs.
	{
		ghOpt := github.PullRequestListOptions{ListOptions: github.ListOptions{PerPage: 1}}
		switch opt.State {
		case issues.OpenState:
			// Do nothing, this is the GitHub default.
		case issues.ClosedState:
			ghOpt.State = "closed"
		}
		ghPRs, ghPRsResp, err := s.cl.PullRequests.List(repo.Owner, repo.Repo, &ghOpt)
		if err != nil {
			return 0, err
		}
		if ghPRsResp.LastPage != 0 {
			count -= uint64(ghPRsResp.LastPage)
		} else {
			count -= uint64(len(ghPRs))
		}
	}

	return count, nil
}

func (s service) Get(_ context.Context, rs issues.RepoSpec, id uint64) (issues.Issue, error) {
	repo := ghRepoSpec(rs)
	issue, _, err := s.cl.Issues.Get(repo.Owner, repo.Repo, int(id))
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    uint64(*issue.Number),
		State: issues.State(*issue.State),
		Title: *issue.Title,
		Comment: issues.Comment{
			User: issues.User{
				Login:     *issue.User.Login,
				AvatarURL: template.URL(*issue.User.AvatarURL),
				HTMLURL:   template.URL(*issue.User.HTMLURL),
			},
			CreatedAt: *issue.CreatedAt,
		},
	}, nil
}

func (s service) ListComments(_ context.Context, rs issues.RepoSpec, id uint64, opt interface{}) ([]issues.Comment, error) {
	repo := ghRepoSpec(rs)
	var comments []issues.Comment

	issue, _, err := s.cl.Issues.Get(repo.Owner, repo.Repo, int(id))
	if err != nil {
		return comments, err
	}
	comments = append(comments, issues.Comment{
		User: issues.User{
			Login:     *issue.User.Login,
			AvatarURL: template.URL(*issue.User.AvatarURL),
			HTMLURL:   template.URL(*issue.User.HTMLURL),
		},
		CreatedAt: *issue.CreatedAt,
		Body:      *issue.Body,
	})

	ghComments, _, err := s.cl.Issues.ListComments(repo.Owner, repo.Repo, int(id), nil) // TODO: Pagination.
	if err != nil {
		return comments, err
	}
	for _, comment := range ghComments {
		comments = append(comments, issues.Comment{
			User: issues.User{
				Login:     *comment.User.Login,
				AvatarURL: template.URL(*comment.User.AvatarURL),
				HTMLURL:   template.URL(*comment.User.HTMLURL),
			},
			CreatedAt: *comment.CreatedAt,
			Body:      *comment.Body,
		})
	}

	return comments, nil
}

func (s service) ListEvents(_ context.Context, rs issues.RepoSpec, id uint64, opt interface{}) ([]issues.Event, error) {
	repo := ghRepoSpec(rs)
	var events []issues.Event

	ghEvents, _, err := s.cl.Issues.ListIssueEvents(repo.Owner, repo.Repo, int(id), nil) // TODO: Pagination.
	if err != nil {
		return events, err
	}
	for _, event := range ghEvents {
		et := issues.EventType(*event.Event)
		if !et.Valid() {
			continue
		}
		e := issues.Event{
			Actor: issues.User{
				Login:     *event.Actor.Login,
				AvatarURL: template.URL(*event.Actor.AvatarURL),
				HTMLURL:   template.URL(*event.Actor.HTMLURL),
			},
			CreatedAt: *event.CreatedAt,
			Type:      et,
		}
		switch et {
		case issues.Renamed:
			e.Rename = &issues.Rename{
				From: *event.Rename.From,
				To:   *event.Rename.To,
			}
		}
		events = append(events, e)
	}

	return events, nil
}

func (s service) CreateComment(_ context.Context, rs issues.RepoSpec, id uint64, c issues.Comment) (issues.Comment, error) {
	repo := ghRepoSpec(rs)
	comment, _, err := s.cl.Issues.CreateComment(repo.Owner, repo.Repo, int(id), &github.IssueComment{
		Body: &c.Body,
	})
	if err != nil {
		return issues.Comment{}, err
	}

	return issues.Comment{
		User: issues.User{
			Login:     *comment.User.Login,
			AvatarURL: template.URL(*comment.User.AvatarURL),
			HTMLURL:   template.URL(*comment.User.HTMLURL),
		},
		CreatedAt: *comment.CreatedAt,
		Body:      *comment.Body,
	}, nil
}

func (s service) Create(_ context.Context, rs issues.RepoSpec, i issues.Issue) (issues.Issue, error) {
	repo := ghRepoSpec(rs)
	issue, _, err := s.cl.Issues.Create(repo.Owner, repo.Repo, &github.IssueRequest{
		Title: &i.Title,
		Body:  &i.Body,
	})
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    uint64(*issue.Number),
		State: issues.State(*issue.State),
		Title: *issue.Title,
		Comment: issues.Comment{
			User: issues.User{
				Login:     *issue.User.Login,
				AvatarURL: template.URL(*issue.User.AvatarURL),
				HTMLURL:   template.URL(*issue.User.HTMLURL),
			},
			CreatedAt: *issue.CreatedAt,
		},
	}, nil
}

func (s service) Edit(_ context.Context, rs issues.RepoSpec, id uint64, ir issues.IssueRequest) (issues.Issue, error) {
	// TODO: Why Validate here but not Create, etc.? Figure this out. Might only be needed in fs implementation.
	if err := ir.Validate(); err != nil {
		return issues.Issue{}, err
	}
	repo := ghRepoSpec(rs)

	ghIR := github.IssueRequest{
		Title: ir.Title,
	}
	if ir.State != nil {
		state := string(*ir.State)
		ghIR.State = &state
	}

	issue, _, err := s.cl.Issues.Edit(repo.Owner, repo.Repo, int(id), &ghIR)
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    uint64(*issue.Number),
		State: issues.State(*issue.State),
		Title: *issue.Title,
		Comment: issues.Comment{
			User: issues.User{
				Login:     *issue.User.Login,
				AvatarURL: template.URL(*issue.User.AvatarURL),
				HTMLURL:   template.URL(*issue.User.HTMLURL),
			},
			CreatedAt: *issue.CreatedAt,
		},
	}, nil
}

func (s service) EditComment(_ context.Context, rs issues.RepoSpec, id uint64, c issues.Comment) (issues.Comment, error) {
	// TODO: Why Validate here but not CreateComment, etc.? Figure this out. Might only be needed in fs implementation.
	if err := c.Validate(); err != nil {
		return issues.Comment{}, err
	}
	repo := ghRepoSpec(rs)

	comment, _, err := s.cl.EditComment(repo.Owner, repo.Repo, int(id), &github.IssueComment{
		Body: &c.Body,
	})
	if err != nil {
		return issues.Comment{}, err
	}

	return issues.Comment{
		User: issues.User{
			Login:     *comment.User.Login,
			AvatarURL: template.URL(*comment.User.AvatarURL),
			HTMLURL:   template.URL(*comment.User.HTMLURL),
		},
		CreatedAt: *comment.CreatedAt,
		Body:      *comment.Body,
	}, nil
}

func (service) CurrentUser(_ context.Context) (issues.User, error) {
	// TODO: Get current user via GH api (if authed), etc.
	return issues.User{
		Login:     "shurcooL",
		AvatarURL: "https://avatars.githubusercontent.com/u/1924134?v=3",
		HTMLURL:   "https://github.com/shurcooL",
	}, nil
}

type repoSpec struct {
	Owner string
	Repo  string
}

func ghRepoSpec(repo issues.RepoSpec) repoSpec {
	ownerRepo := strings.Split(repo.URI, "/")
	if len(ownerRepo) != 2 {
		panic(fmt.Errorf(`RepoSpec is not of form "owner/repo": %v`, repo))
	}
	return repoSpec{
		Owner: ownerRepo[0],
		Repo:  ownerRepo[1],
	}
}
