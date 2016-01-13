// Package fs implements issues.Service using a filesystem.
package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
	"src.sourcegraph.com/apps/tracker/issues"
)

// NewService creates a virtual filesystem-backed issues.Service using root for storage.
/*func NewService(root webdav.FileSystem) issues.Service {
	return service{
		fs: root,
	}
}

type service struct {
	fs webdav.FileSystem
}*/

// TODO.
// NewService creates a filesystem-backed issues.Service rooted at rootDir.
func NewService(rootDir string) issues.Service {
	return service{
		root: rootDir,
	}
}

type service struct {
	// root directory for issue storage for all repos.
	root string
}

// TODO.
func (s service) namespace(repoURI string) webdav.FileSystem {
	return webdav.Dir(filepath.Join(s.root, "repo", filepath.FromSlash(repoURI), "tracker"))
}
func (s service) createNamespace(repoURI string) error {
	// Only needed for first issue in the repo.
	// TODO: Can this be better?
	return os.MkdirAll(filepath.Join(s.root, "repo", filepath.FromSlash(repoURI), "tracker", issuesDir), 0755)
}

func (s service) List(ctx context.Context, repo issues.RepoSpec, opt issues.IssueListOptions) ([]issues.Issue, error) {
	fs := s.namespace(repo.URI)

	var is []issues.Issue

	dirs, err := readDirIDs(fs, issuesDir)
	if err != nil {
		return is, err
	}
	for i := len(dirs); i > 0; i-- {
		dir := dirs[i-1]
		if !dir.IsDir() {
			continue
		}

		var issue issue
		err = jsonDecodeFile(fs, issueCommentPath(dir.ID, 0), &issue)
		if err != nil {
			return is, err
		}

		if opt.State != issues.AllStates && issue.State != issues.State(opt.State) {
			continue
		}

		// Count comments.
		comments, err := readDirIDs(fs, issueDir(dir.ID))
		if err != nil {
			return is, err
		}
		user := issues.UserSpec{ID: uint64(issue.AuthorUID)}
		is = append(is, issues.Issue{
			ID:    dir.ID,
			State: issue.State,
			Title: issue.Title,
			Comment: issues.Comment{
				User:      sgUser(ctx, user),
				CreatedAt: issue.CreatedAt,
			},
			Replies: len(comments) - 1,
		})
	}

	return is, nil
}

func (s service) Count(ctx context.Context, repo issues.RepoSpec, opt issues.IssueListOptions) (uint64, error) {
	fs := s.namespace(repo.URI)

	var count uint64

	dirs, err := readDirIDs(fs, issuesDir)
	if err != nil {
		return 0, err
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		var issue issue
		err = jsonDecodeFile(fs, issueCommentPath(dir.ID, 0), &issue)
		if err != nil {
			return 0, err
		}

		if opt.State != issues.AllStates && issue.State != issues.State(opt.State) {
			continue
		}

		count++
	}

	return count, nil
}

func (s service) Get(ctx context.Context, repo issues.RepoSpec, id uint64) (issues.Issue, error) {
	currentUser := (*issues.UserSpec)(nil) // TODO.

	fs := s.namespace(repo.URI)

	var issue issue
	err := jsonDecodeFile(fs, issueCommentPath(id, 0), &issue)
	if err != nil {
		return issues.Issue{}, err
	}

	user := issues.UserSpec{ID: uint64(issue.AuthorUID)}

	return issues.Issue{
		ID:    id,
		State: issue.State,
		Title: issue.Title,
		Comment: issues.Comment{
			User:      sgUser(ctx, user),
			CreatedAt: issue.CreatedAt,
			Editable:  nil == canEdit(ctx, currentUser, issue.AuthorUID),
		},
	}, nil
}

func (s service) ListComments(ctx context.Context, repo issues.RepoSpec, id uint64, opt interface{}) ([]issues.Comment, error) {
	currentUser := (*issues.UserSpec)(nil) // TODO.

	fs := s.namespace(repo.URI)

	var comments []issues.Comment

	fis, err := readDirIDs(fs, issueDir(id))
	if err != nil {
		return comments, err
	}
	for _, fi := range fis {
		var comment comment
		err = jsonDecodeFile(fs, issueCommentPath(id, fi.ID), &comment)
		if err != nil {
			return comments, err
		}

		user := issues.UserSpec{ID: uint64(comment.AuthorUID)}
		comments = append(comments, issues.Comment{
			ID:        fi.ID,
			User:      sgUser(ctx, user),
			CreatedAt: comment.CreatedAt,
			Body:      comment.Body,
			Editable:  nil == canEdit(ctx, currentUser, comment.AuthorUID),
		})
	}

	return comments, nil
}

func (s service) ListEvents(ctx context.Context, repo issues.RepoSpec, id uint64, opt interface{}) ([]issues.Event, error) {
	fs := s.namespace(repo.URI)

	var events []issues.Event

	fis, err := readDirIDs(fs, issueEventsDir(id))
	if err != nil {
		return events, err
	}
	for _, fi := range fis {
		var event event
		err = jsonDecodeFile(fs, issueEventPath(id, fi.ID), &event)
		if err != nil {
			return events, err
		}

		user := issues.UserSpec{ID: uint64(event.ActorUID)}
		events = append(events, issues.Event{
			ID:        fi.ID,
			Actor:     sgUser(ctx, user),
			CreatedAt: event.CreatedAt,
			Type:      event.Type,
			Rename:    event.Rename,
		})
	}

	return events, nil
}

func (s service) CreateComment(ctx context.Context, repo issues.RepoSpec, id uint64, c issues.Comment) (issues.Comment, error) {
	// CreateComment operation requires an authenticated user with read access.
	currentUser := (*issues.UserSpec)(nil) // TODO.
	if currentUser == nil {
		return issues.Comment{}, os.ErrPermission
	}

	if err := c.Validate(); err != nil {
		return issues.Comment{}, err
	}

	fs := s.namespace(repo.URI)

	comment := comment{
		AuthorUID: int32(currentUser.ID),
		CreatedAt: time.Now().UTC(),
		Body:      c.Body,
	}

	user := issues.UserSpec{ID: uint64(comment.AuthorUID)}

	// Commit to storage.
	commentID, err := nextID(fs, issueDir(id))
	if err != nil {
		return issues.Comment{}, err
	}
	err = jsonEncodeFile(fs, issueCommentPath(id, commentID), comment)
	if err != nil {
		return issues.Comment{}, err
	}

	return issues.Comment{
		ID:        commentID,
		User:      sgUser(ctx, user),
		CreatedAt: comment.CreatedAt,
		Body:      comment.Body,
		Editable:  true, // You can always edit comments you've created.
	}, nil
}

func (s service) Create(ctx context.Context, repo issues.RepoSpec, i issues.Issue) (issues.Issue, error) {
	// Create operation requires an authenticated user with read access.
	currentUser := (*issues.UserSpec)(nil) // TODO.
	if currentUser == nil {
		return issues.Issue{}, os.ErrPermission
	}

	if err := i.Validate(); err != nil {
		return issues.Issue{}, err
	}

	if i.Reference != nil {
		return issues.Issue{}, errors.New("Reference not supported in fs service implementation")
	}

	if err := s.createNamespace(repo.URI); err != nil {
		return issues.Issue{}, err
	}
	fs := s.namespace(repo.URI)

	issue := issue{
		State: issues.OpenState,
		Title: i.Title,
		comment: comment{
			AuthorUID: int32(currentUser.ID),
			CreatedAt: time.Now().UTC(),
			Body:      i.Body,
		},
	}

	user := issues.UserSpec{ID: uint64(issue.AuthorUID)}

	// Commit to storage.
	issueID, err := nextID(fs, issuesDir)
	if err != nil {
		return issues.Issue{}, err
	}
	err = fs.Mkdir(issueDir(issueID), 0755)
	if err != nil {
		return issues.Issue{}, err
	}
	err = fs.Mkdir(issueEventsDir(issueID), 0755)
	if err != nil {
		return issues.Issue{}, err
	}
	err = jsonEncodeFile(fs, issueCommentPath(issueID, 0), issue)
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    issueID,
		State: issue.State,
		Title: issue.Title,
		Comment: issues.Comment{
			ID:        0,
			User:      sgUser(ctx, user),
			CreatedAt: issue.CreatedAt,
			Body:      issue.Body,
			Editable:  true, // You can always edit issues you've created.
		},
	}, nil
}

// canEdit returns nil error if currentUser is authorized to edit an entry created by authorUID.
// It returns os.ErrPermission or an error that happened in other cases.
func canEdit(ctx context.Context, currentUser *issues.UserSpec, authorUID int32) error {
	if currentUser == nil {
		// Not logged in, cannot edit anything.
		return os.ErrPermission
	}
	if int32(currentUser.ID) == authorUID {
		// If you're the author, you can always edit it.
		return nil
	}
	authInfo := struct{ Write bool }{} // TODO.
	switch authInfo.Write {
	case true:
		// If you have write access (or greater), you can edit.
		return nil
	default:
		return os.ErrPermission
	}
}

func (s service) Edit(ctx context.Context, repo issues.RepoSpec, id uint64, ir issues.IssueRequest) (issues.Issue, error) {
	currentUser := (*issues.UserSpec)(nil) // TODO.
	if currentUser == nil {
		return issues.Issue{}, os.ErrPermission
	}

	if err := ir.Validate(); err != nil {
		return issues.Issue{}, err
	}

	fs := s.namespace(repo.URI)

	// Get from storage.
	var issue issue
	err := jsonDecodeFile(fs, issueCommentPath(id, 0), &issue)
	if err != nil {
		return issues.Issue{}, err
	}

	// Authorization check.
	if err := canEdit(ctx, currentUser, issue.AuthorUID); err != nil {
		return issues.Issue{}, err
	}

	// TODO: Doing this here before committing in case it fails; think about factoring this out into a user service that augments...
	user := issues.UserSpec{ID: uint64(issue.AuthorUID)}

	// Apply edits.
	if ir.State != nil {
		issue.State = *ir.State
	}
	origTitle := issue.Title
	if ir.Title != nil {
		issue.Title = *ir.Title
	}

	// Commit to storage.
	err = jsonEncodeFile(fs, issueCommentPath(id, 0), issue)
	if err != nil {
		return issues.Issue{}, err
	}

	// THINK: Is this the best place to do this? Should it be returned from this func? How would GH backend do it?
	// Create event and commit to storage.
	event := event{
		ActorUID:  int32(currentUser.ID),
		CreatedAt: time.Now().UTC(),
	}
	switch {
	case ir.State != nil && *ir.State == issues.OpenState:
		event.Type = issues.Reopened
	case ir.State != nil && *ir.State == issues.ClosedState:
		event.Type = issues.Closed
	case ir.Title != nil:
		event.Type = issues.Renamed
		event.Rename = &issues.Rename{
			From: origTitle,
			To:   *ir.Title,
		}
	}
	eventID, err := nextID(fs, issueEventsDir(id))
	if err != nil {
		return issues.Issue{}, err
	}
	err = jsonEncodeFile(fs, issueEventPath(id, eventID), event)
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    id,
		State: issue.State,
		Title: issue.Title,
		Comment: issues.Comment{
			ID:        0,
			User:      sgUser(ctx, user),
			CreatedAt: issue.CreatedAt,
			Editable:  true, // You can always edit issues you've edited.
		},
	}, nil
}

func (s service) EditComment(ctx context.Context, repo issues.RepoSpec, id uint64, cr issues.CommentRequest) (issues.Comment, error) {
	currentUser := (*issues.UserSpec)(nil) // TODO.
	if currentUser == nil {
		return issues.Comment{}, os.ErrPermission
	}

	if _, err := cr.Validate(); err != nil {
		return issues.Comment{}, err
	}

	if cr.Body == nil {
		return issues.Comment{}, errors.New("unsupported EditComment request for fs service implementation")
	}

	fs := s.namespace(repo.URI)

	if cr.ID == 0 {
		// Get from storage.
		var issue issue
		err := jsonDecodeFile(fs, issueCommentPath(id, 0), &issue)
		if err != nil {
			return issues.Comment{}, err
		}

		// Authorization check.
		if err := canEdit(ctx, currentUser, issue.AuthorUID); err != nil {
			return issues.Comment{}, err
		}

		// TODO: Doing this here before committing in case it fails; think about factoring this out into a user service that augments...
		user := issues.UserSpec{ID: uint64(issue.AuthorUID)}

		// Apply edits.
		issue.Body = *cr.Body

		// Commit to storage.
		err = jsonEncodeFile(fs, issueCommentPath(id, 0), issue)
		if err != nil {
			return issues.Comment{}, err
		}

		return issues.Comment{
			ID:        0,
			User:      sgUser(ctx, user),
			CreatedAt: issue.CreatedAt,
			Body:      issue.Body,
			Editable:  true, // You can always edit comments you've edited.
		}, nil
	}

	// Get from storage.
	var comment comment
	err := jsonDecodeFile(fs, issueCommentPath(id, cr.ID), &comment)
	if err != nil {
		return issues.Comment{}, err
	}

	// Authorization check.
	if err := canEdit(ctx, currentUser, comment.AuthorUID); err != nil {
		return issues.Comment{}, err
	}

	// TODO: Doing this here before committing in case it fails; think about factoring this out into a user service that augments...
	user := issues.UserSpec{ID: uint64(comment.AuthorUID)}

	// Apply edits.
	comment.Body = *cr.Body

	// Commit to storage.
	err = jsonEncodeFile(fs, issueCommentPath(id, cr.ID), comment)
	if err != nil {
		return issues.Comment{}, err
	}

	return issues.Comment{
		ID:        cr.ID,
		User:      sgUser(ctx, user),
		CreatedAt: comment.CreatedAt,
		Body:      comment.Body,
		Editable:  true, // You can always edit comments you've edited.
	}, nil
}

// nextID returns the next id for the given dir. If there are no previous elements, it begins with id 1.
func nextID(fs webdav.FileSystem, dir string) (uint64, error) {
	fis, err := readDirIDs(fs, dir)
	if err != nil {
		return 0, err
	}
	if len(fis) == 0 {
		return 1, nil
	}
	return fis[len(fis)-1].ID + 1, nil
}

func (s service) Search(_ context.Context, opt issues.SearchOptions) (issues.SearchResponse, error) {
	return issues.SearchResponse{}, errors.New("Search endpoint not implemented in fs service implementation")
}

// TODO.
func (service) CurrentUser(ctx context.Context) (*issues.User, error) {
	user := issues.UserSpec{ID: uint64(0)}
	u := sgUser(ctx, user)
	return &u, nil
}

func formatUint64(n uint64) string { return strconv.FormatUint(n, 10) }
