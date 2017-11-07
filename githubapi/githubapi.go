// Package githubapi implements issues.Service using GitHub API client.
package githubapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"github.com/shurcooL/issues"
	"github.com/shurcooL/notifications"
	"github.com/shurcooL/users"
	ghusers "github.com/shurcooL/users/githubapi"
)

// NewService creates a GitHub-backed issues.Service using given GitHub client.
// It uses notifications service, if not nil. At this time it infers the current user
// from the client (its authentication info), and cannot be used to serve multiple users.
func NewService(client *github.Client, notifications notifications.ExternalService) (issues.Service, error) {
	if client == nil {
		client = github.NewClient(nil)
	}
	users, err := ghusers.NewService(client)
	if err != nil {
		return nil, err
	}
	currentUser, err := users.GetAuthenticated(context.Background())
	if err != nil {
		return nil, err
	}
	return service{
		cl:            client,
		notifications: notifications,
		currentUser:   currentUser,
	}, nil
}

type service struct {
	cl *github.Client

	// notifications may be nil if there's no notifications service.
	notifications notifications.ExternalService

	currentUser users.User
}

// We use 0 as a special ID for the comment that is the issue description. This comment is edited differently.
const issueDescriptionCommentID uint64 = 0

func (s service) List(ctx context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) ([]issues.Issue, error) {
	repo, err := ghRepoSpec(rs)
	if err != nil {
		// TODO: Map ghRepoSpec to HTTP 400 Bad Request error somehow... Else it becomes 500.
		return nil, err
	}
	ghOpt := github.IssueListByRepoOptions{}
	switch opt.State {
	case issues.StateFilter(issues.OpenState):
		// Do nothing, this is the GitHub default.
	case issues.StateFilter(issues.ClosedState):
		ghOpt.State = "closed"
	case issues.AllStates:
		ghOpt.State = "all"
	}
	ghIssuesAndPRs, _, err := s.cl.Issues.ListByRepo(ctx, repo.Owner, repo.Repo, &ghOpt)
	if err != nil {
		return nil, err
	}

	var is []issues.Issue
	for _, issue := range ghIssuesAndPRs {
		// Filter out PRs.
		if issue.IsPullRequest() {
			continue
		}

		var labels []issues.Label
		for _, l := range issue.Labels {
			labels = append(labels, issues.Label{
				Name:  *l.Name,
				Color: ghColor(*l.Color),
			})
		}
		is = append(is, issues.Issue{
			ID:     uint64(*issue.Number),
			State:  issues.State(*issue.State),
			Title:  *issue.Title,
			Labels: labels,
			Comment: issues.Comment{
				User:      ghUser(issue.User),
				CreatedAt: *issue.CreatedAt,
			},
			Replies: *issue.Comments,
		})
	}

	return is, nil
}

func (s service) Count(ctx context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) (uint64, error) {
	repo, err := ghRepoSpec(rs)
	if err != nil {
		return 0, err
	}
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
		ghIssuesAndPRs, ghIssuesAndPRsResp, err := s.cl.Issues.ListByRepo(ctx, repo.Owner, repo.Repo, &ghOpt)
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
		ghPRs, ghPRsResp, err := s.cl.PullRequests.List(ctx, repo.Owner, repo.Repo, &ghOpt)
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

// canEdit returns nil error if currentUser is authorized to edit an entry created by authorID.
// It returns os.ErrPermission or an error that happened in other cases.
func (s service) canEdit(isCollaborator bool, isCollaboratorErr error, authorID int) error {
	if s.currentUser.ID == 0 {
		// Not logged in, cannot edit anything.
		return os.ErrPermission
	}
	if s.currentUser.ID == uint64(authorID) {
		// If you're the author, you can always edit it.
		return nil
	}
	if isCollaboratorErr != nil {
		return isCollaboratorErr
	}
	switch isCollaborator {
	case true:
		// If you have write access (or greater), you can edit.
		return nil
	default:
		return os.ErrPermission
	}
}

func (s service) Get(ctx context.Context, rs issues.RepoSpec, id uint64) (issues.Issue, error) {
	repo, err := ghRepoSpec(rs)
	if err != nil {
		return issues.Issue{}, err
	}
	issue, _, err := s.cl.Issues.Get(ctx, repo.Owner, repo.Repo, int(id))
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

	// TODO, THINK: Where's the best place for this? It should be inside canEdit, but don't want to
	//              do it more than 1 per service call. Perhaps store/check inside request context?
	//
	//              In here it doesn't matter since Get only calls canEdit once; but it matters for
	//              ListComments because it has canEdit inside a for loop.
	isCollaborator, _, isCollaboratorErr := s.cl.Repositories.IsCollaborator(ctx, repo.Owner, repo.Repo, s.currentUser.Login)

	// TODO: Eliminate comment body properties from issues.Issue. It's missing increasingly more fields, like Edited, etc.
	return issues.Issue{
		ID:    uint64(*issue.Number),
		State: issues.State(*issue.State),
		Title: *issue.Title,
		Comment: issues.Comment{
			User:      ghUser(issue.User),
			CreatedAt: *issue.CreatedAt,
			Editable:  nil == s.canEdit(isCollaborator, isCollaboratorErr, *issue.User.ID),
		},
	}, nil
}

func (s service) ListComments(ctx context.Context, rs issues.RepoSpec, id uint64, opt *issues.ListOptions) ([]issues.Comment, error) {
	// TODO: Respect opt.Start and opt.Length, if given.

	repo, err := ghRepoSpec(rs)
	if err != nil {
		return nil, err
	}
	var comments []issues.Comment

	// TODO, THINK: Where's the best place for this? It should be inside canEdit, but don't want to
	//              do it more than 1 per service call. Perhaps store/check inside request context?
	isCollaborator, _, isCollaboratorErr := s.cl.Repositories.IsCollaborator(ctx, repo.Owner, repo.Repo, s.currentUser.Login)

	issue, _, err := s.cl.Issues.Get(ctx, repo.Owner, repo.Repo, int(id))
	if err != nil {
		return comments, err
	}
	issueReactions, err := s.listIssueReactions(ctx, repo.Owner, repo.Repo, int(id))
	if err != nil {
		return comments, err
	}
	reactions, err := s.reactions(issueReactions)
	if err != nil {
		return comments, err
	}
	var edited *issues.Edited
	/* TODO: Get the actual edited information once GitHub API allows it. Can't use issue.UpdatedAt because of false positives, since it includes the entire issue, not just its comment body.
	if !issue.UpdatedAt.Equal(*issue.CreatedAt) {
		edited = &issues.Edited{
			By: users.User{Login: "Someone"}, //ghUser(issue.Actor), // TODO: Get the actual actor once GitHub API allows it.
			At: *issue.UpdatedAt,
		}
	}*/
	comments = append(comments, issues.Comment{
		ID:        issueDescriptionCommentID,
		User:      ghUser(issue.User),
		CreatedAt: *issue.CreatedAt,
		Edited:    edited,
		Body:      *issue.Body,
		Reactions: reactions,
		Editable:  nil == s.canEdit(isCollaborator, isCollaboratorErr, *issue.User.ID),
	})

	ghOpt := &github.IssueListCommentsOptions{}
	for {
		ghComments, resp, err := s.cl.Issues.ListComments(ctx, repo.Owner, repo.Repo, int(id), ghOpt)
		if err != nil {
			return comments, err
		}
		for _, comment := range ghComments {
			commentReactions, err := s.listIssueCommentReactions(ctx, repo.Owner, repo.Repo, *comment.ID)
			if err != nil {
				return comments, err
			}
			reactions, err := s.reactions(commentReactions)
			if err != nil {
				return comments, err
			}
			var edited *issues.Edited
			if !comment.UpdatedAt.Equal(*comment.CreatedAt) {
				edited = &issues.Edited{
					By: users.User{Login: "Someone"}, //ghUser(comment.Actor), // TODO: Get the actual actor once GitHub API allows it.
					At: *comment.UpdatedAt,
				}
			}
			comments = append(comments, issues.Comment{
				ID:        uint64(*comment.ID),
				User:      ghUser(comment.User),
				CreatedAt: *comment.CreatedAt,
				Edited:    edited,
				Body:      *comment.Body,
				Reactions: reactions,
				Editable:  nil == s.canEdit(isCollaborator, isCollaboratorErr, *comment.User.ID),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		ghOpt.ListOptions.Page = resp.NextPage
	}

	return comments, nil
}

func (s service) ListEvents(ctx context.Context, rs issues.RepoSpec, id uint64, opt *issues.ListOptions) ([]issues.Event, error) {
	// TODO: Pagination. Respect opt.Start and opt.Length, if given.

	repo, err := ghRepoSpec(rs)
	if err != nil {
		return nil, err
	}
	var events []issues.Event

	ghEvents, _, err := s.cl.Issues.ListIssueEvents(ctx, repo.Owner, repo.Repo, int(id), nil)
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
		case issues.Closed:
			e.Close = issues.Close{
				CommitID:      *event.CommitID,
				CommitHTMLURL: "", // TODO: Implement (being mindful that the commit can be in a different repository).
			}
		case issues.Renamed:
			e.Rename = &issues.Rename{
				From: *event.Rename.From,
				To:   *event.Rename.To,
			}
		case issues.Labeled, issues.Unlabeled:
			e.Label = &issues.Label{
				Name:  *event.Label.Name,
				Color: ghColor(*event.Label.Color),
			}
		}
		events = append(events, e)
	}

	return events, nil
}

func (s service) CreateComment(ctx context.Context, rs issues.RepoSpec, id uint64, c issues.Comment) (issues.Comment, error) {
	repo, err := ghRepoSpec(rs)
	if err != nil {
		return issues.Comment{}, err
	}
	comment, _, err := s.cl.Issues.CreateComment(ctx, repo.Owner, repo.Repo, int(id), &github.IssueComment{
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

func (s service) Create(ctx context.Context, rs issues.RepoSpec, i issues.Issue) (issues.Issue, error) {
	repo, err := ghRepoSpec(rs)
	if err != nil {
		return issues.Issue{}, err
	}
	issue, _, err := s.cl.Issues.Create(ctx, repo.Owner, repo.Repo, &github.IssueRequest{
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

func (s service) Edit(ctx context.Context, rs issues.RepoSpec, id uint64, ir issues.IssueRequest) (issues.Issue, []issues.Event, error) {
	// TODO: Why Validate here but not Create, etc.? Figure this out. Might only be needed in fs implementation.
	if err := ir.Validate(); err != nil {
		return issues.Issue{}, nil, err
	}
	repo, err := ghRepoSpec(rs)
	if err != nil {
		return issues.Issue{}, nil, err
	}

	ghIR := github.IssueRequest{
		Title: ir.Title,
	}
	if ir.State != nil {
		state := string(*ir.State)
		ghIR.State = &state
	}

	issue, _, err := s.cl.Issues.Edit(ctx, repo.Owner, repo.Repo, int(id), &ghIR)
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
	repo, err := ghRepoSpec(rs)
	if err != nil {
		return issues.Comment{}, err
	}

	if cr.ID == issueDescriptionCommentID {
		var comment issues.Comment

		// Apply edits.
		if cr.Body != nil {
			// Use Issues.Edit() API to edit comment 0 (the issue description).
			issue, _, err := s.cl.Issues.Edit(ctx, repo.Owner, repo.Repo, int(id), &github.IssueRequest{
				Body: cr.Body,
			})
			if err != nil {
				return issues.Comment{}, err
			}

			var edited *issues.Edited
			/* TODO: Get the actual edited information once GitHub API allows it. Can't use issue.UpdatedAt because of false positives, since it includes the entire issue, not just its comment body.
			if !issue.UpdatedAt.Equal(*issue.CreatedAt) {
				edited = &issues.Edited{
					By: users.User{Login: "Someone"}, //ghUser(issue.Actor), // TODO: Get the actual actor once GitHub API allows it.
					At: *issue.UpdatedAt,
				}
			}*/
			// TODO: Consider setting reactions? But it's semi-expensive (to fetch all user details) and not used by app...
			comment.ID = issueDescriptionCommentID
			comment.User = ghUser(issue.User)
			comment.CreatedAt = *issue.CreatedAt
			comment.Edited = edited
			comment.Body = *issue.Body
			comment.Editable = true // You can always edit comments you've edited.
		}
		if cr.Reaction != nil {
			// Toggle reaction by trying to create it, and if it already existed, then remove it.
			reaction, resp, err := s.cl.Reactions.CreateIssueReaction(ctx, repo.Owner, repo.Repo, int(id), externalizeReaction(*cr.Reaction))
			if err != nil {
				return issues.Comment{}, err
			}
			if resp.StatusCode == http.StatusOK {
				// If we got 200 instead of 201, we should be removing the reaction instead.
				_, err := s.cl.Reactions.DeleteReaction(ctx, *reaction.ID)
				if err != nil {
					return issues.Comment{}, err
				}
			}

			issueReactions, err := s.listIssueReactions(ctx, repo.Owner, repo.Repo, int(id))
			if err != nil {
				return issues.Comment{}, err
			}
			reactions, err := s.reactions(issueReactions)
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
		ghComment, _, err := s.cl.Issues.EditComment(ctx, repo.Owner, repo.Repo, int(cr.ID), &github.IssueComment{
			Body: cr.Body,
		})
		if err != nil {
			return issues.Comment{}, err
		}

		var edited *issues.Edited
		if !ghComment.UpdatedAt.Equal(*ghComment.CreatedAt) {
			edited = &issues.Edited{
				By: users.User{Login: "Someone"}, //ghUser(ghComment.Actor), // TODO: Get the actual actor once GitHub API allows it.
				At: *ghComment.UpdatedAt,
			}
		}
		// TODO: Consider setting reactions? But it's semi-expensive (to fetch all user details) and not used by app...
		comment.ID = uint64(*ghComment.ID)
		comment.User = ghUser(ghComment.User)
		comment.CreatedAt = *ghComment.CreatedAt
		comment.Edited = edited
		comment.Body = *ghComment.Body
		comment.Editable = true // You can always edit comments you've edited.
	}
	if cr.Reaction != nil {
		// Toggle reaction by trying to create it, and if it already existed, then remove it.
		reaction, resp, err := s.cl.Reactions.CreateIssueCommentReaction(ctx, repo.Owner, repo.Repo, int(cr.ID), externalizeReaction(*cr.Reaction))
		if err != nil {
			return issues.Comment{}, err
		}
		if resp.StatusCode == http.StatusOK {
			// If we got 200 instead of 201, we should be removing the reaction instead.
			_, err := s.cl.Reactions.DeleteReaction(ctx, *reaction.ID)
			if err != nil {
				return issues.Comment{}, err
			}
		}

		commentReactions, err := s.listIssueCommentReactions(ctx, repo.Owner, repo.Repo, int(cr.ID))
		if err != nil {
			return issues.Comment{}, err
		}
		reactions, err := s.reactions(commentReactions)
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

func ghRepoSpec(repo issues.RepoSpec) (repoSpec, error) {
	// TODO, THINK: Include "github.com/" prefix or not?
	//              So far I'm leaning towards "yes", because it's more definitive and matches
	//              local uris that also include host. This way, the host can be checked as part of
	//              request, rather than kept implicit.
	ghOwnerRepo := strings.Split(repo.URI, "/")
	if len(ghOwnerRepo) != 3 || ghOwnerRepo[0] != "github.com" || ghOwnerRepo[1] == "" || ghOwnerRepo[2] == "" {
		return repoSpec{}, fmt.Errorf(`RepoSpec is not of form "github.com/owner/repo": %q`, repo.URI)
	}
	return repoSpec{
		Owner: ghOwnerRepo[1],
		Repo:  ghOwnerRepo[2],
	}, nil
}

func ghUser(user *github.User) users.User {
	return users.User{
		UserSpec: users.UserSpec{
			ID:     uint64(*user.ID),
			Domain: "github.com",
		},
		Login:     *user.Login,
		AvatarURL: *user.AvatarURL,
		HTMLURL:   *user.HTMLURL,
	}
}

// ghColor converts a GitHub color hex string like "ff0000"
// into an issues.RGB value.
func ghColor(hex string) issues.RGB {
	var c issues.RGB
	fmt.Sscanf(hex, "%02x%02x%02x", &c.R, &c.G, &c.B)
	return c
}
