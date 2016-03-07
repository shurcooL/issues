// Package fs implements issues.Service using a filesystem.
package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/shurcooL/users"
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
func NewService(rootDir string, users users.Service, usersDomain string) issues.Service {
	return service{
		root:        rootDir,
		users:       users,
		usersDomain: usersDomain,
	}
}

type service struct {
	// root directory for issue storage for all repos.
	root string

	users users.Service

	// HACK: Temporary parameter acts as a hint for which domain the "old" users come from.
	usersDomain string
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
		author := issues.UserSpec{ID: uint64(issue.AuthorUID), Domain: s.usersDomain}
		is = append(is, issues.Issue{
			ID:    dir.ID,
			State: issue.State,
			Title: issue.Title,
			Comment: issues.Comment{
				User:      s.issuesUser(ctx, author),
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

	author := issues.UserSpec{ID: uint64(issue.AuthorUID), Domain: s.usersDomain}

	return issues.Issue{
		ID:    id,
		State: issue.State,
		Title: issue.Title,
		Comment: issues.Comment{
			User:      s.issuesUser(ctx, author),
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

		author := issues.UserSpec{ID: uint64(comment.AuthorUID), Domain: s.usersDomain}
		var reactions []issues.Reaction
		for _, cr := range comment.Reactions {
			reaction := issues.Reaction{
				Reaction: cr.EmojiID,
			}
			for _, uid := range cr.AuthorUIDs {
				// TODO: Since we're potentially getting many of the same users multiple times here, consider caching them locally.
				reactionAuthor := issues.UserSpec{ID: uint64(uid), Domain: s.usersDomain}
				reaction.Users = append(reaction.Users, s.issuesUser(ctx, reactionAuthor))
			}
			reactions = append(reactions, reaction)
		}
		comments = append(comments, issues.Comment{
			ID:        fi.ID,
			User:      s.issuesUser(ctx, author),
			CreatedAt: comment.CreatedAt,
			Body:      comment.Body,
			Reactions: reactions,
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

		actor := issues.UserSpec{ID: uint64(event.ActorUID), Domain: s.usersDomain}
		events = append(events, issues.Event{
			ID:        fi.ID,
			Actor:     s.issuesUser(ctx, actor),
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

	author := issues.UserSpec{ID: uint64(comment.AuthorUID), Domain: s.usersDomain}

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
		User:      s.issuesUser(ctx, author),
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

	author := issues.UserSpec{ID: uint64(issue.AuthorUID), Domain: s.usersDomain}

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
			User:      s.issuesUser(ctx, author),
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

// canReact returns nil error if currentUser is authorized to react to an entry.
// It returns os.ErrPermission or an error that happened in other cases.
func canReact(currentUser *issues.UserSpec) error {
	if currentUser == nil {
		// Not logged in, cannot react to anything.
		return os.ErrPermission
	}
	return nil
}

func (s service) Edit(ctx context.Context, repo issues.RepoSpec, id uint64, ir issues.IssueRequest) (issues.Issue, []issues.Event, error) {
	currentUser := (*issues.UserSpec)(nil) // TODO.
	if currentUser == nil {
		return issues.Issue{}, nil, os.ErrPermission
	}

	if err := ir.Validate(); err != nil {
		return issues.Issue{}, nil, err
	}

	fs := s.namespace(repo.URI)

	// Get from storage.
	var issue issue
	err := jsonDecodeFile(fs, issueCommentPath(id, 0), &issue)
	if err != nil {
		return issues.Issue{}, nil, err
	}

	// Authorization check.
	if err := canEdit(ctx, currentUser, issue.AuthorUID); err != nil {
		return issues.Issue{}, nil, err
	}

	// TODO: Doing this here before committing in case it fails; think about factoring this out into a user service that augments...
	author := issues.UserSpec{ID: uint64(issue.AuthorUID), Domain: s.usersDomain}
	actor := *currentUser

	// Apply edits.
	origState := issue.State
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
		return issues.Issue{}, nil, err
	}

	// Create event and commit to storage.
	createdAt := time.Now().UTC()
	event := event{
		ActorUID:  int32(currentUser.ID),
		CreatedAt: createdAt,
	}
	// TODO: A single edit operation can result in multiple events, we should emit multiple events in such cases. We're currently emitting at most one event.
	switch {
	case ir.State != nil && *ir.State != origState:
		switch *ir.State {
		case issues.OpenState:
			event.Type = issues.Reopened
		case issues.ClosedState:
			event.Type = issues.Closed
		}
	case ir.Title != nil && *ir.Title != origTitle:
		event.Type = issues.Renamed
		event.Rename = &issues.Rename{
			From: origTitle,
			To:   *ir.Title,
		}
	}
	var events []issues.Event
	if event.Type != "" {
		eventID, err := nextID(fs, issueEventsDir(id))
		if err != nil {
			return issues.Issue{}, nil, err
		}
		err = jsonEncodeFile(fs, issueEventPath(id, eventID), event)
		if err != nil {
			return issues.Issue{}, nil, err
		}

		events = append(events, issues.Event{
			ID:        eventID,
			Actor:     s.issuesUser(ctx, actor),
			CreatedAt: event.CreatedAt,
			Type:      event.Type,
			Rename:    event.Rename,
		})
	}

	return issues.Issue{
		ID:    id,
		State: issue.State,
		Title: issue.Title,
		Comment: issues.Comment{
			ID:        0,
			User:      s.issuesUser(ctx, author),
			CreatedAt: issue.CreatedAt,
			Editable:  true, // You can always edit issues you've edited.
		},
	}, events, nil
}

func (s service) EditComment(ctx context.Context, repo issues.RepoSpec, id uint64, cr issues.CommentRequest) (issues.Comment, error) {
	currentUser := (*issues.UserSpec)(nil) // TODO.
	if currentUser == nil {
		return issues.Comment{}, os.ErrPermission
	}

	requiresEdit, err := cr.Validate()
	if err != nil {
		return issues.Comment{}, err
	}

	if cr.Body == nil {
		return issues.Comment{}, errors.New("unsupported EditComment request for fs service implementation")
	}

	fs := s.namespace(repo.URI)

	// TODO: Merge these 2 cases (first comment aka issue vs reply comments) into one.
	if cr.ID == 0 {
		// Get from storage.
		var issue issue
		err := jsonDecodeFile(fs, issueCommentPath(id, 0), &issue)
		if err != nil {
			return issues.Comment{}, err
		}

		// Authorization check.
		switch requiresEdit {
		case true:
			if err := canEdit(ctx, currentUser, issue.AuthorUID); err != nil {
				return issues.Comment{}, err
			}
		case false:
			if err := canReact(currentUser); err != nil {
				return issues.Comment{}, err
			}
		}

		// TODO: Doing this here before committing in case it fails; think about factoring this out into a user service that augments...
		author := issues.UserSpec{ID: uint64(issue.AuthorUID), Domain: s.usersDomain}

		// Apply edits.
		if cr.Body != nil {
			issue.Body = *cr.Body
		}
		if cr.Reaction != nil {
			toggleReaction(&issue.comment, int32(currentUser.ID), *cr.Reaction)
		}

		// Commit to storage.
		err = jsonEncodeFile(fs, issueCommentPath(id, 0), issue)
		if err != nil {
			return issues.Comment{}, err
		}

		return issues.Comment{
			ID:        0,
			User:      s.issuesUser(ctx, author),
			CreatedAt: issue.CreatedAt,
			Body:      issue.Body,
			Editable:  true, // You can always edit comments you've edited.
		}, nil
	}

	// Get from storage.
	var comment comment
	err = jsonDecodeFile(fs, issueCommentPath(id, cr.ID), &comment)
	if err != nil {
		return issues.Comment{}, err
	}

	// Authorization check.
	switch requiresEdit {
	case true:
		if err := canEdit(ctx, currentUser, comment.AuthorUID); err != nil {
			return issues.Comment{}, err
		}
	case false:
		if err := canReact(currentUser); err != nil {
			return issues.Comment{}, err
		}
	}

	// TODO: Doing this here before committing in case it fails; think about factoring this out into a user service that augments...
	author := issues.UserSpec{ID: uint64(comment.AuthorUID), Domain: s.usersDomain}

	// Apply edits.
	if cr.Body != nil {
		comment.Body = *cr.Body
	}
	if cr.Reaction != nil {
		toggleReaction(&comment, int32(currentUser.ID), *cr.Reaction)
	}

	// Commit to storage.
	err = jsonEncodeFile(fs, issueCommentPath(id, cr.ID), comment)
	if err != nil {
		return issues.Comment{}, err
	}

	return issues.Comment{
		ID:        cr.ID,
		User:      s.issuesUser(ctx, author),
		CreatedAt: comment.CreatedAt,
		Body:      comment.Body,
		Editable:  true, // You can always edit comments you've edited.
	}, nil
}

// toggleReaction toggles reaction emojiID to comment c for specified user uid.
func toggleReaction(c *comment, uid int32, emojiID issues.EmojiID) {
	for i := range c.Reactions {
		if c.Reactions[i].EmojiID == emojiID {
			// Toggle this user's reaction.
			switch reactedUID := contains(c.Reactions[i].AuthorUIDs, uid); {
			case reactedUID == -1:
				// Add this reaction.
				c.Reactions[i].AuthorUIDs = append(c.Reactions[i].AuthorUIDs, uid)
			default:
				// Remove this reaction. Delete without preserving order.
				c.Reactions[i].AuthorUIDs[reactedUID] = c.Reactions[i].AuthorUIDs[len(c.Reactions[i].AuthorUIDs)-1]
				c.Reactions[i].AuthorUIDs = c.Reactions[i].AuthorUIDs[:len(c.Reactions[i].AuthorUIDs)-1]

				// If there are no more authors backing it, this reaction goes away.
				if len(c.Reactions[i].AuthorUIDs) == 0 {
					c.Reactions, c.Reactions[len(c.Reactions)-1] = append(c.Reactions[:i], c.Reactions[i+1:]...), reaction{} // Delete preserving order.
				}
			}
			return
		}
	}

	// If we get here, this is the first reaction of its kind.
	// Add it to the end of the list.
	c.Reactions = append(c.Reactions,
		reaction{
			EmojiID:    emojiID,
			AuthorUIDs: []int32{uid},
		},
	)
}

// contains returns index of e in set, or -1 if it's not there.
func contains(set []int32, e int32) int {
	for i, v := range set {
		if v == e {
			return i
		}
	}
	return -1
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
func (s service) CurrentUser(ctx context.Context) (*issues.User, error) {
	/*user := issues.UserSpec{ID: uint64(0), Domain: s.usersDomain}
	u := s.issuesUser(ctx, user)
	return &u, nil*/
	return nil, nil
}

func formatUint64(n uint64) string { return strconv.FormatUint(n, 10) }
