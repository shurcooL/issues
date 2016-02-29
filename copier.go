package fs

import (
	"fmt"

	"golang.org/x/net/context"
	"src.sourcegraph.com/apps/tracker/issues"
)

var _ issues.CopierFrom = service{}

func (s service) CopyFrom(ctx context.Context, src issues.Service, repo issues.RepoSpec) error {
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
		// Copy issue.
		issue := issue{
			State: i.State,
			Title: i.Title,
			comment: comment{
				AuthorUID: int32(i.User.ID),
				CreatedAt: i.CreatedAt,
				// TODO: This doesn't work, Get doesn't return body, reactions, etc.; using ListComments for now for that... Improve this.
				//       Perhaps this is motivation to separate Comment out of Issue, so get can return only Issue and it's clear that Comment details are elsewhere.
				//       Just leave non-comment-specific things in Issue like Author and CreatedAt, but remove Body, Reactions, etc., those belong to comment only.
				//       That would also make the distinction between reactions to first issue comment (its body) and reactions to entire issue (i.e. in list view), if that's ever desireable.
				//       However, it would just mean that Create endpoint would likely need to create an issue and then a comment (2 storage ops rater than 1), but that's completely fair.
				//Body:      i.Body,
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
		// Issue will be created as part of first comment, since we need to embed its comment too.

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
			for _, r := range c.Reactions {
				reaction := reaction{
					EmojiID: r.Reaction,
				}
				for _, u := range r.Users {
					reaction.AuthorUIDs = append(reaction.AuthorUIDs, int32(u.ID))
				}
				comment.Reactions = append(comment.Reactions, reaction)
			}

			if c.ID == 0 {
				issue.comment = comment

				// Put in storage.
				err = jsonEncodeFile(fs, issueCommentPath(i.ID, 0), issue)
				if err != nil {
					return err
				}
				continue
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

	fmt.Println("All done.")
	return nil
}
