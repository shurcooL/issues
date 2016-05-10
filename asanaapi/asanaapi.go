// Package asanaapi implements issues.Service using Asana API client.
package asanaapi

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"github.com/shurcooL/issues"
	"github.com/shurcooL/users"
	"github.com/tambet/go-asana/asana"
	"golang.org/x/net/context"
)

// NewService creates a Asana-backed issues.Service using given Asana client.
// At this time it infers the current user from the client (its authentication info), and cannot be used to serve multiple users.
func NewService(client *asana.Client, users users.Service) issues.Service {
	if client == nil {
		client = asana.NewClient(nil)
	}

	s := service{
		cl:    client,
		users: users,
	}

	s.currentUser, s.currentUserErr = s.users.GetAuthenticatedSpec(context.TODO())

	return s
}

type service struct {
	cl *asana.Client

	users users.Service

	currentUser    users.UserSpec
	currentUserErr error
}

func atoi(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func (s service) List(_ context.Context, rs issues.RepoSpec, opt issues.IssueListOptions) ([]issues.Issue, error) {
	var tasks []asana.Task
	var err error
	switch opt.State {
	case issues.StateFilter(issues.OpenState):
		tasks, err = s.cl.ListProjectTasks(atoi(rs.URI), &asana.Filter{CompletedSince: "now", OptFields: []string{"name", "created_at", "created_by.(name|photo.image_128x128)", "hearts"}})
		if err != nil {
			return nil, err
		}
	case issues.StateFilter(issues.ClosedState):
		// TODO: Filter out complete?
		tasks, err = s.cl.ListProjectTasks(atoi(rs.URI), &asana.Filter{OptFields: []string{"completed"}})
		if err != nil {
			return nil, err
		}
	case issues.AllStates:
		tasks, err = s.cl.ListProjectTasks(atoi(rs.URI), &asana.Filter{OptFields: []string{"completed"}})
		if err != nil {
			return nil, err
		}
	}

	var is []issues.Issue
	for _, task := range tasks {
		is = append(is, issues.Issue{
			ID:    uint64(task.ID),
			State: state(task),
			Title: task.Name,
			Comment: issues.Comment{
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

func (s service) Get(ctx context.Context, rs issues.RepoSpec, id uint64) (issues.Issue, error) {
	task, err := s.cl.GetTask(int64(id), &asana.Filter{OptFields: []string{"created_at", "created_by.(name|photo.image_128x128)", "name", "hearts.user.name"}})
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    uint64(task.ID),
		State: state(task),
		Title: task.Name,
		Comment: issues.Comment{
			User:      asanaUser(task.CreatedBy),
			CreatedAt: task.CreatedAt,
			Editable:  false, // TODO.
		},
	}, nil
}

func (s service) ListComments(ctx context.Context, rs issues.RepoSpec, id uint64, opt interface{}) ([]issues.Comment, error) {
	var comments []issues.Comment

	task, err := s.cl.GetTask(int64(id), &asana.Filter{OptFields: []string{"created_at", "created_by.(name|photo.image_128x128)", "name", "hearts.user.name", "notes"}})
	if err != nil {
		return comments, err
	}
	var reactions []issues.Reaction
	if len(task.Hearts) > 0 {
		reaction := issues.Reaction{
			Reaction: "heart",
		}
		for _, heart := range task.Hearts {
			reaction.Users = append(reaction.Users, asanaUser(heart.User))
		}
		reactions = append(reactions, reaction)
	}
	comments = append(comments, issues.Comment{
		ID:        uint64(task.ID),
		User:      asanaUser(task.CreatedBy),
		CreatedAt: task.CreatedAt,
		Body:      task.Notes,
		Reactions: reactions,
		Editable:  false, // TODO.
	})

	stories, err := s.cl.ListTaskStories(int64(id), &asana.Filter{OptFields: []string{"created_at", "created_by.(name|photo.image_128x128)", "hearts.user.name", "text", "type"}})
	if err != nil {
		return comments, err
	}
	for _, story := range stories {
		if story.Type != "comment" {
			continue
		}

		var reactions []issues.Reaction
		if len(story.Hearts) > 0 {
			reaction := issues.Reaction{
				Reaction: "heart",
			}
			for _, heart := range story.Hearts {
				reaction.Users = append(reaction.Users, asanaUser(heart.User))
			}
			reactions = append(reactions, reaction)
		}
		comments = append(comments, issues.Comment{
			ID:        uint64(story.ID),
			User:      asanaUser(story.CreatedBy),
			CreatedAt: story.CreatedAt,
			Body:      story.Text,
			Reactions: reactions,
			Editable:  false, // TODO.
		})
	}

	return comments, nil
}

func (s service) ListEvents(_ context.Context, rs issues.RepoSpec, id uint64, opt interface{}) ([]issues.Event, error) {
	stories, err := s.cl.ListTaskStories(int64(id), &asana.Filter{OptExpand: []string{"created_by"}})
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
		case story.Text == "marked this task complete":
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

func (s service) EditComment(_ context.Context, rs issues.RepoSpec, id uint64, cr issues.CommentRequest) (issues.Comment, error) {
	// TODO.
	return issues.Comment{}, fmt.Errorf("EditComment: not implemented")
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
		AvatarURL: template.URL(user.Photo["image_128x128"]),
	}
}
