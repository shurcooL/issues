// Package asanaapi implements issues.Service using Asana API client.
package asanaapi

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shurcooL/issues"
	"github.com/shurcooL/reactions"
	"github.com/shurcooL/users"
	anusers "github.com/shurcooL/users/asanaapi"
	"github.com/tambet/go-asana/asana"
)

// NewService creates a Asana-backed issues.Service using given Asana client.
// At this time it infers the current user from the client (its authentication info),
// and cannot be used to serve multiple users.
func NewService(client *asana.Client) (issues.Service, error) {
	if client == nil {
		client = asana.NewClient(nil)
	}
	users, err := anusers.NewService(client)
	if err != nil {
		return nil, err
	}
	currentUser, err := users.GetAuthenticatedSpec(context.Background())
	if err != nil {
		return nil, err
	}
	return service{
		cl:          client,
		currentUser: currentUser,
	}, nil
}

type service struct {
	cl *asana.Client

	currentUser users.UserSpec
}

// We use 0 as a special ID for the comment that is the issue description. This comment is edited differently.
const issueDescriptionCommentID uint64 = 0

func atoi(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func (s service) List(ctx context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) ([]issues.Issue, error) {
	var tasks []asana.Task
	var err error
	switch opt.State {
	case issues.StateFilter(issues.OpenState):
		tasks, err = s.cl.ListProjectTasks(ctx, atoi(rs.URI), &asana.Filter{CompletedSince: "now", OptFields: []string{"name", "created_at", "created_by.(name|photo.image_128x128)", "hearts"}})
		if err != nil {
			return nil, err
		}
	case issues.StateFilter(issues.ClosedState):
		// TODO: Filter out complete?
		tasks, err = s.cl.ListProjectTasks(ctx, atoi(rs.URI), &asana.Filter{OptFields: []string{"completed", "name", "created_at", "created_by.(name|photo.image_128x128)", "hearts"}})
		if err != nil {
			return nil, err
		}
	case issues.AllStates:
		tasks, err = s.cl.ListProjectTasks(ctx, atoi(rs.URI), &asana.Filter{OptFields: []string{"completed", "name", "created_at", "created_by.(name|photo.image_128x128)", "hearts"}})
		if err != nil {
			return nil, err
		}
	}

	var is []issues.Issue
	for _, task := range tasks {
		if opt.State == issues.StateFilter(issues.ClosedState) && !task.Completed {
			continue
		}

		is = append(is, issues.Issue{
			ID:    uint64(task.ID),
			State: state(task),
			Title: task.Name,
			Comment: issues.Comment{
				ID:        issueDescriptionCommentID,
				User:      asanaUser(task.CreatedBy),
				CreatedAt: task.CreatedAt,
			},
			//Replies: *issue.Comments,
		})
	}

	return is, nil
}

func (s service) Count(_ context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) (uint64, error) {
	// TODO.
	return 0, nil
}

func state(task asana.Task) issues.State {
	switch task.Completed {
	case false:
		return issues.OpenState
	case true:
		return issues.ClosedState
	default:
		panic("unreachable")
	}
}

func (s service) Get(ctx context.Context, _ issues.RepoSpec, id uint64) (issues.Issue, error) {
	task, err := s.cl.GetTask(ctx, int64(id), &asana.Filter{OptFields: []string{"completed", "created_at", "created_by.(name|photo.image_128x128)", "name", "hearts.user.name"}})
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    uint64(task.ID),
		State: state(task),
		Title: task.Name,
		Comment: issues.Comment{
			ID:        issueDescriptionCommentID,
			User:      asanaUser(task.CreatedBy),
			CreatedAt: task.CreatedAt,
			Editable:  false, // TODO.
		},
	}, nil
}

// canEdit returns nil error if currentUser is authorized to edit an entry created by authorID.
// It returns os.ErrPermission or an error that happened in other cases.
func (s service) canEdit(isCollaborator bool, isCollaboratorErr error, authorID int64) error {
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

func (s service) ListComments(ctx context.Context, _ issues.RepoSpec, id uint64, opt *issues.ListOptions) ([]issues.Comment, error) {
	// TODO: Pagination. Respect opt.Start and opt.Length, if given.

	var comments []issues.Comment

	// TODO: Figure this out.
	var (
		isCollaborator    = true
		isCollaboratorErr error
	)

	task, err := s.cl.GetTask(ctx, int64(id), &asana.Filter{OptFields: []string{"created_at", "created_by.(name|photo.image_128x128)", "name", "hearts.user.name", "notes"}})
	if err != nil {
		return comments, err
	}
	var rs []reactions.Reaction
	if len(task.Hearts) > 0 {
		reaction := reactions.Reaction{
			Reaction: "heart",
		}
		for _, heart := range task.Hearts {
			reaction.Users = append(reaction.Users, asanaUser(heart.User))
		}
		rs = append(rs, reaction)
	}
	comments = append(comments, issues.Comment{
		ID:        issueDescriptionCommentID,
		User:      asanaUser(task.CreatedBy),
		CreatedAt: task.CreatedAt,
		Body:      task.Notes,
		Reactions: rs,
		Editable:  nil == s.canEdit(isCollaborator, isCollaboratorErr, task.CreatedBy.ID),
	})

	stories, err := s.cl.ListTaskStories(ctx, int64(id), &asana.Filter{OptFields: []string{"created_at", "created_by.(name|photo.image_128x128)", "hearts.user.name", "text", "type"}})
	if err != nil {
		return comments, err
	}
	for _, story := range stories {
		if story.Type != "comment" {
			continue
		}

		var rs []reactions.Reaction
		if len(story.Hearts) > 0 {
			reaction := reactions.Reaction{
				Reaction: "heart",
			}
			for _, heart := range story.Hearts {
				reaction.Users = append(reaction.Users, asanaUser(heart.User))
			}
			rs = append(rs, reaction)
		}
		comments = append(comments, issues.Comment{
			ID:        uint64(story.ID),
			User:      asanaUser(story.CreatedBy),
			CreatedAt: story.CreatedAt,
			Body:      story.Text,
			Reactions: rs,
			Editable:  false, // TODO.
		})
	}

	return comments, nil
}

func (s service) ListEvents(ctx context.Context, _ issues.RepoSpec, id uint64, opt *issues.ListOptions) ([]issues.Event, error) {
	// TODO: Pagination. Respect opt.Start and opt.Length, if given.

	stories, err := s.cl.ListTaskStories(ctx, int64(id), &asana.Filter{OptExpand: []string{"created_by"}})
	if err != nil {
		return nil, err
	}

	var events []issues.Event
	for _, story := range stories {
		if story.Type != "system" {
			continue
		}

		var et issues.EventType
		switch {
		case story.Text == "marked incomplete":
			et = issues.Reopened
		case story.Text == "marked this task complete" || story.Text == "completed this task":
			et = issues.Closed
		case strings.HasPrefix(story.Text, "added to ") || strings.HasPrefix(story.Text, "♥ "):
			continue
		default:
			et = issues.EventType(story.Text)
		}
		// TODO.
		/*if !et.Valid() {
			continue
		}*/

		event := issues.Event{
			ID:        uint64(story.ID),
			Actor:     asanaUser(story.CreatedBy),
			CreatedAt: story.CreatedAt,
			Type:      et,
		}
		events = append(events, event)
	}

	return events, nil
}

func (s service) CreateComment(_ context.Context, rs issues.RepoSpec, id uint64, c issues.Comment) (issues.Comment, error) {
	// TODO.
	return issues.Comment{}, fmt.Errorf("CreateComment: not implemented")
}

func (s service) Create(_ context.Context, rs issues.RepoSpec, i issues.Issue) (issues.Issue, error) {
	// TODO.
	return issues.Issue{}, fmt.Errorf("Create: not implemented")
}

func (s service) Edit(_ context.Context, rs issues.RepoSpec, id uint64, ir issues.IssueRequest) (issues.Issue, []issues.Event, error) {
	// TODO.
	return issues.Issue{}, nil, fmt.Errorf("Edit: not implemented")
}

func (s service) EditComment(ctx context.Context, rs issues.RepoSpec, id uint64, cr issues.CommentRequest) (issues.Comment, error) {
	if _, err := cr.Validate(); err != nil {
		return issues.Comment{}, err
	}
	// TODO: Move into internal (Asana-specific) CommentRequest validation?
	if cr.Reaction != nil && *cr.Reaction != "heart" { // The only allowed emoji by Asana.
		return issues.Comment{}, fmt.Errorf("reaction with emoji %q is not supported", *cr.Reaction)
	}
	// TODO: Check that one of (cr.Body != nil || cr.Reaction != nil) is true?

	if cr.ID == issueDescriptionCommentID {
		tu := asana.TaskUpdate{
			Notes: cr.Body,
		}
		if cr.Reaction != nil && *cr.Reaction == "heart" {
			hearted := true // TODO: Figure out the true/false value...
			tu.Hearted = &hearted
		}
		task, err := s.cl.UpdateTask(ctx, int64(id), tu, &asana.Filter{OptFields: []string{"created_at", "created_by.(name|photo.image_128x128)", "name", "hearts.user.name", "notes"}})
		if err != nil {
			return issues.Comment{}, err
		}
		var rs []reactions.Reaction
		if len(task.Hearts) > 0 {
			reaction := reactions.Reaction{
				Reaction: "heart",
			}
			for _, heart := range task.Hearts {
				reaction.Users = append(reaction.Users, asanaUser(heart.User))
			}
			rs = append(rs, reaction)
		}
		return issues.Comment{
			ID:        issueDescriptionCommentID,
			User:      asanaUser(task.CreatedBy),
			CreatedAt: task.CreatedAt,
			Body:      task.Notes,
			Reactions: rs,
			Editable:  true, // You can always edit comments you've edited.
		}, nil
	}

	// TODO.
	return issues.Comment{}, fmt.Errorf("EditComment(id %v, cr.ID %v): not implemented", id, cr.ID)
}

func asanaUser(user asana.User) users.User {
	return users.User{
		UserSpec: users.UserSpec{
			ID:     uint64(user.ID),
			Domain: "app.asana.com",
		},
		Login:     user.Name,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.Photo["image_128x128"],
	}
}
