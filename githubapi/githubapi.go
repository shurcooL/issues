// Package githubapi implements issues.Service using GitHub API client.
package githubapi

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"github.com/shurcooL/issues"
	"github.com/shurcooL/notifications"
	"github.com/shurcooL/users"
	"golang.org/x/net/context"
)

// NewService creates a GitHub-backed issues.Service using given GitHub client,
// and notifications for external API of Notifications Center app, if any.
// At this time it infers the current user from the client (its authentication info), and cannot be used to serve multiple users.
func NewService(client *github.Client, notifications notifications.ExternalService, users users.Service) issues.Service {
	if client == nil {
		client = github.NewClient(nil)
	}

	s := service{
		cl:            client,
		notifications: notifications,
		users:         users,
	}

	s.currentUser, s.currentUserErr = s.users.GetAuthenticated(context.TODO())

	return s
}

type service struct {
	cl *github.Client

	// notifications is the external API of Notifications Center app.
	// Its value is nil if Notification Center app is not started.
	notifications notifications.ExternalService

	users users.Service

	currentUser    users.User
	currentUserErr error
}

// We use 0 as a special ID for the comment that is the issue description. This comment is edited differently.
const issueDescriptionCommentID = uint64(0)

func (s service) List(_ context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) ([]issues.Issue, error) {
	repo := ghRepoSpec(rs)
	ghOpt := github.IssueListByRepoOptions{}
	switch opt.State {
	case issues.StateFilter(issues.OpenState):
		// Do nothing, this is the GitHub default.
	case issues.StateFilter(issues.ClosedState):
		ghOpt.State = "closed"
	case issues.AllStates:
		ghOpt.State = "all"
	}
	ghIssuesAndPRs, _, err := s.cl.Issues.ListByRepo(repo.Owner, repo.Repo, &ghOpt)
	if err != nil {
		return nil, err
	}

	var is []issues.Issue
	for _, issue := range ghIssuesAndPRs {
		// Filter out PRs.
		if issue.PullRequestLinks != nil && issue.PullRequestLinks.URL != nil {
			continue
		}

		is = append(is, issues.Issue{
			ID:    uint64(*issue.Number),
			State: issues.State(*issue.State),
			Title: *issue.Title,
			Comment: issues.Comment{
				User:      ghUser(issue.User),
				CreatedAt: *issue.CreatedAt,
			},
			Replies: *issue.Comments,
		})
	}

	return is, nil
}

func (s service) Count(_ context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) (uint64, error) {
	repo := ghRepoSpec(rs)
	var ghState string
	switch opt.State {
	case issues.StateFilter(issues.OpenState):
		// Do nothing, this is the GitHub default.
	case issues.StateFilter(issues.ClosedState):
		ghState = "closed"
	case issues.AllStates:
		ghState = "all"
	}

	var count uint64

	// Count Issues and PRs (since there appears to be no way to count just issues in GitHub API).
	{
		ghOpt := github.IssueListByRepoOptions{
			State:       ghState,
			ListOptions: github.ListOptions{PerPage: 1},
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
		ghOpt := github.PullRequestListOptions{
			State:       ghState,
			ListOptions: github.ListOptions{PerPage: 1},
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

// canEdit returns nil error if currentUser is authorized to edit an entry created by authorUID.
// It returns os.ErrPermission or an error that happened in other cases.
func (s service) canEdit(repo repoSpec, authorID int) error {
	if s.currentUser.ID == 0 {
		// Not logged in, cannot edit anything.
		return os.ErrPermission
	}
	if s.currentUser.ID == uint64(authorID) {
		// If you're the author, you can always edit it.
		return nil
	}
	isCollaborator, _, err := s.cl.Repositories.IsCollaborator(repo.Owner, repo.Repo, s.currentUser.Login)
	if err != nil {
		return err
	}
	switch isCollaborator {
	case true:
		// If you have write access (or greater), you can edit.
		return nil
	default:
		return os.ErrPermission
	}
}

// TODO: Dedup.
func parseIssueSpec(issueAPIURL string) (issues.RepoSpec, int, error) {
	u, err := url.Parse(issueAPIURL)
	if err != nil {
		return issues.RepoSpec{}, 0, err
	}
	e := strings.Split(u.Path, "/")
	if len(e) < 5 {
		return issues.RepoSpec{}, 0, fmt.Errorf("unexpected path (too few elements): %q", u.Path)
	}
	if got, want := e[len(e)-2], "issues"; got != want {
		return issues.RepoSpec{}, 0, fmt.Errorf(`unexpected path element %q, expecting %q`, got, want)
	}
	id, err := strconv.Atoi(e[len(e)-1])
	if err != nil {
		return issues.RepoSpec{}, 0, err
	}
	return issues.RepoSpec{URI: e[len(e)-4] + "/" + e[len(e)-3]}, id, nil
}

func (s service) Get(ctx context.Context, rs issues.RepoSpec, id uint64) (issues.Issue, error) {
	repo := ghRepoSpec(rs)
	issue, _, err := s.cl.Issues.Get(repo.Owner, repo.Repo, int(id))
	if err != nil {
		return issues.Issue{}, err
	}

	if s.currentUser.ID != 0 {
		// Mark as read.
		err = s.markRead(ctx, rs, id)
		if err != nil {
			log.Println("service.Get: failed to markRead:", err)
		}
	}

	return issues.Issue{
		ID:    uint64(*issue.Number),
		State: issues.State(*issue.State),
		Title: *issue.Title,
		Comment: issues.Comment{
			User:      ghUser(issue.User),
			CreatedAt: *issue.CreatedAt,
			Editable:  nil == s.canEdit(repo, *issue.User.ID),
		},
	}, nil
}

func (s service) ListComments(ctx context.Context, rs issues.RepoSpec, id uint64, opt interface{}) ([]issues.Comment, error) {
	repo := ghRepoSpec(rs)
	var comments []issues.Comment

	issue, _, err := s.cl.Issues.Get(repo.Owner, repo.Repo, int(id))
	if err != nil {
		return comments, err
	}
	issueReactions, err := s.listIssueReactions(repo.Owner, repo.Repo, int(id))
	if err != nil {
		return comments, err
	}
	reactions, err := s.reactions(ctx, issueReactions)
	if err != nil {
		return comments, err
	}
	comments = append(comments, issues.Comment{
		ID:        issueDescriptionCommentID,
		User:      ghUser(issue.User),
		CreatedAt: *issue.CreatedAt,
		Body:      *issue.Body,
		Reactions: reactions,
		Editable:  nil == s.canEdit(repo, *issue.User.ID),
	})

	ghOpt := &github.IssueListCommentsOptions{}
	for {
		ghComments, resp, err := s.cl.Issues.ListComments(repo.Owner, repo.Repo, int(id), ghOpt)
		if err != nil {
			return comments, err
		}
		for _, comment := range ghComments {
			commentReactions, err := s.listIssueCommentReactions(repo.Owner, repo.Repo, int(*comment.ID))
			if err != nil {
				return comments, err
			}
			reactions, err := s.reactions(ctx, commentReactions)
			if err != nil {
				return comments, err
			}
			comments = append(comments, issues.Comment{
				ID:        uint64(*comment.ID),
				User:      ghUser(comment.User),
				CreatedAt: *comment.CreatedAt,
				Body:      *comment.Body,
				Reactions: reactions,
				Editable:  nil == s.canEdit(repo, *comment.User.ID),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		ghOpt.ListOptions.Page = resp.NextPage
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
			ID:        uint64(*event.ID),
			Actor:     ghUser(event.Actor),
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
		ID:        uint64(*comment.ID),
		User:      ghUser(comment.User),
		CreatedAt: *comment.CreatedAt,
		Body:      *comment.Body,
		Editable:  true, // You can always edit comments you've created.
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
			ID:        issueDescriptionCommentID,
			User:      ghUser(issue.User),
			CreatedAt: *issue.CreatedAt,
			Editable:  true, // You can always edit issues you've created.
		},
	}, nil
}

func (s service) Edit(_ context.Context, rs issues.RepoSpec, id uint64, ir issues.IssueRequest) (issues.Issue, []issues.Event, error) {
	// TODO: Why Validate here but not Create, etc.? Figure this out. Might only be needed in fs implementation.
	if err := ir.Validate(); err != nil {
		return issues.Issue{}, nil, err
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
		return issues.Issue{}, nil, err
	}

	// GitHub API doesn't return the event that will be generated as a result, so we predict what it'll be.
	event := issues.Event{
		// TODO: Figure out if event ID needs to be set, and if so, how to best do that...
		Actor:     s.currentUser, // Only logged in users can edit, so we're guaranteed to have a current user.
		CreatedAt: *issue.CreatedAt,
	}
	// TODO: A single edit operation can result in multiple events, we should emit multiple events in such cases. We're currently emitting at most one event.
	switch {
	case ir.State != nil: // TODO: && *ir.State != origState:
		switch *ir.State {
		case issues.OpenState:
			event.Type = issues.Reopened
		case issues.ClosedState:
			event.Type = issues.Closed
		}
	case ir.Title != nil: // TODO: && *ir.Title != origTitle:
		event.Type = issues.Renamed
		event.Rename = &issues.Rename{
			From: "", // TODO: origTitle,
			To:   *ir.Title,
		}
	}
	var events []issues.Event
	if event.Type != "" {
		events = append(events, event)
	}

	return issues.Issue{
		ID:    uint64(*issue.Number),
		State: issues.State(*issue.State),
		Title: *issue.Title,
		Comment: issues.Comment{
			ID:        issueDescriptionCommentID,
			User:      ghUser(issue.User),
			CreatedAt: *issue.CreatedAt,
			Editable:  true, // You can always edit issues you've edited.
		},
	}, events, nil
}

func (s service) EditComment(ctx context.Context, rs issues.RepoSpec, id uint64, cr issues.CommentRequest) (issues.Comment, error) {
	// TODO: Why Validate here but not CreateComment, etc.? Figure this out. Might only be needed in fs implementation.
	if _, err := cr.Validate(); err != nil {
		return issues.Comment{}, err
	}
	repo := ghRepoSpec(rs)

	if cr.ID == issueDescriptionCommentID {
		var comment issues.Comment

		// Apply edits.
		if cr.Body != nil {
			// Use Issues.Edit() API to edit comment 0 (the issue description).
			issue, _, err := s.cl.Issues.Edit(repo.Owner, repo.Repo, int(id), &github.IssueRequest{
				Body: cr.Body,
			})
			if err != nil {
				return issues.Comment{}, err
			}

			// TODO: Consider setting reactions? But it's semi-expensive (to fetch all user details) and not used by app...
			comment.ID = issueDescriptionCommentID
			comment.User = ghUser(issue.User)
			comment.CreatedAt = *issue.CreatedAt
			comment.Body = *issue.Body
			comment.Editable = true // You can always edit comments you've edited.
		}
		if cr.Reaction != nil {
			// Toggle reaction by trying to create it, and if it already existed, then remove it.
			reaction, resp, err := s.cl.Reactions.CreateIssueReaction(repo.Owner, repo.Repo, int(id), externalizeReaction(*cr.Reaction))
			if err != nil {
				return issues.Comment{}, err
			}
			if resp.StatusCode == http.StatusOK {
				// If we got 200 instead of 201, we should be removing the reaction instead.
				_, err := s.cl.Reactions.DeleteReaction(*reaction.ID)
				if err != nil {
					return issues.Comment{}, err
				}
			}

			issueReactions, err := s.listIssueReactions(repo.Owner, repo.Repo, int(id))
			if err != nil {
				return issues.Comment{}, err
			}
			reactions, err := s.reactions(ctx, issueReactions)
			if err != nil {
				return issues.Comment{}, err
			}

			// TODO: Consider setting other fields? But it's semi-expensive (another API call) and not used by app...
			comment.Reactions = reactions
		}

		return comment, nil
	}

	var comment issues.Comment

	// Apply edits.
	if cr.Body != nil {
		// GitHub API uses comment ID and doesn't need issue ID. Comment IDs are unique per repo (rather than per issue).
		ghComment, _, err := s.cl.Issues.EditComment(repo.Owner, repo.Repo, int(cr.ID), &github.IssueComment{
			Body: cr.Body,
		})
		if err != nil {
			return issues.Comment{}, err
		}

		// TODO: Consider setting reactions? But it's semi-expensive (to fetch all user details) and not used by app...
		comment.ID = uint64(*ghComment.ID)
		comment.User = ghUser(ghComment.User)
		comment.CreatedAt = *ghComment.CreatedAt
		comment.Body = *ghComment.Body
		comment.Editable = true // You can always edit comments you've edited.
	}
	if cr.Reaction != nil {
		// Toggle reaction by trying to create it, and if it already existed, then remove it.
		reaction, resp, err := s.cl.Reactions.CreateIssueCommentReaction(repo.Owner, repo.Repo, int(cr.ID), externalizeReaction(*cr.Reaction))
		if err != nil {
			return issues.Comment{}, err
		}
		if resp.StatusCode == http.StatusOK {
			// If we got 200 instead of 201, we should be removing the reaction instead.
			_, err := s.cl.Reactions.DeleteReaction(*reaction.ID)
			if err != nil {
				return issues.Comment{}, err
			}
		}

		commentReactions, err := s.listIssueCommentReactions(repo.Owner, repo.Repo, int(cr.ID))
		if err != nil {
			return issues.Comment{}, err
		}
		reactions, err := s.reactions(ctx, commentReactions)
		if err != nil {
			return issues.Comment{}, err
		}

		// TODO: Consider setting other fields? But it's semi-expensive (another API call) and not used by app...
		comment.Reactions = reactions
	}

	return comment, nil
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

func ghUser(user *github.User) users.User {
	return users.User{
		UserSpec: users.UserSpec{
			ID:     uint64(*user.ID),
			Domain: "github.com",
		},
		Login:     *user.Login,
		AvatarURL: template.URL(*user.AvatarURL),
		HTMLURL:   template.URL(*user.HTMLURL),
	}
}
