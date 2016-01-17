package fs

import (
	"fmt"

	"golang.org/x/net/context"
	"src.sourcegraph.com/apps/tracker/issues"
)

func (s service) CopyFrom(src issues.Service, repo issues.RepoSpec) error {
	ctx := context.TODO()
	if err := s.createNamespace(repo.URI); err != nil {
		return err
	}
	fs := s.namespace(repo.URI)

	is, err := src.List(ctx, repo, issues.IssueListOptions{State: issues.AllStates})
	if err != nil {
		return err
	}
	fmt.Printf("Copying %v issues.\n", len(is))
	for _, i := range is {
		i, err = src.Get(ctx, repo, i.ID) // Needed to get the body, since List operation doesn't include all details.
		if err != nil {
			return err
		}
		{
			// Copy issue.
			issue := issue{
				State: i.State,
				Title: i.Title,
				comment: comment{
					AuthorUID: int32(i.User.ID),
					CreatedAt: i.CreatedAt,
					Body:      i.Body,
				},
			}

			// Put in storage.
			err = fs.Mkdir(issueDir(i.ID), 0755)
			if err != nil {
				return err
			}
			err = fs.Mkdir(issueEventsDir(i.ID), 0755)
			if err != nil {
				return err
			}
			err = jsonEncodeFile(fs, issueCommentPath(i.ID, 0), issue)
			if err != nil {
				return err
			}
		}

		comments, err := src.ListComments(ctx, repo, i.ID, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Issue %v: Copying %v comments.\n", i.ID, len(comments))
		for _, c := range comments {
			// Copy comment.
			comment := comment{
				AuthorUID: int32(c.User.ID),
				CreatedAt: c.CreatedAt,
				Body:      c.Body,
			}

			// Put in storage.
			err = jsonEncodeFile(fs, issueCommentPath(i.ID, c.ID), comment)
			if err != nil {
				return err
			}
		}

		events, err := src.ListEvents(ctx, repo, i.ID, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Issue %v: Copying %v events.\n", i.ID, len(events))
		for _, e := range events {
			// Copy event.
			event := event{
				ActorUID:  int32(e.Actor.ID),
				CreatedAt: e.CreatedAt,
				Type:      e.Type,
				Rename:    e.Rename,
			}

			// Put in storage.
			err = jsonEncodeFile(fs, issueEventPath(i.ID, e.ID), event)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
