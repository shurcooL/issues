package githubapi

import (
	"context"

	"github.com/google/go-github/github"
	"github.com/shurcooL/reactions"
	"github.com/shurcooL/users"
)

// reactions converts a []*github.Reaction to []reactions.Reaction.
func (s service) reactions(ghReactions []*github.Reaction) ([]reactions.Reaction, error) {
	var rs []reactions.Reaction
	var m = make(map[string]int) // EmojiID -> rs index.
	for _, reaction := range ghReactions {
		i, ok := m[*reaction.Content]
		if !ok {
			// First reaction of its type, add to rs and m.
			rs = append(rs, reactions.Reaction{
				Reaction: internalizeReaction(*reaction.Content),
			})
			i = len(rs) - 1
			m[*reaction.Content] = i
		}

		// Only return the details of first few users and authed user.
		const expandUsers = 10
		isAuthedUser := s.currentUser.ID != 0 && uint64(*reaction.User.ID) == s.currentUser.ID
		if len(rs[i].Users) < expandUsers || isAuthedUser {
			rs[i].Users = append(rs[i].Users, ghUser(reaction.User))
		} else {
			rs[i].Users = append(rs[i].Users, users.User{})
		}
	}
	return rs, nil
}

// listIssueReactions fetches all pages.
func (s service) listIssueReactions(ctx context.Context, owner, repo string, id int) ([]*github.Reaction, error) {
	var issueReactions []*github.Reaction
	ghOpt := &github.ListOptions{}
	for {
		irs, resp, err := s.cl.Reactions.ListIssueReactions(ctx, owner, repo, id, ghOpt)
		if err != nil {
			return nil, err
		}
		issueReactions = append(issueReactions, irs...)
		if resp.NextPage == 0 {
			break
		}
		ghOpt.Page = resp.NextPage
	}
	return issueReactions, nil
}

// listIssueCommentReactions fetches all pages.
func (s service) listIssueCommentReactions(ctx context.Context, owner, repo string, id int) ([]*github.Reaction, error) {
	var commentReactions []*github.Reaction
	ghOpt := &github.ListOptions{}
	for {
		crs, resp, err := s.cl.Reactions.ListIssueCommentReactions(ctx, owner, repo, id, ghOpt)
		if err != nil {
			return nil, err
		}
		commentReactions = append(commentReactions, crs...)
		if resp.NextPage == 0 {
			break
		}
		ghOpt.Page = resp.NextPage
	}
	return commentReactions, nil
}

// internalizeReaction converts github.Reaction.Content to reactions.EmojiID.
func internalizeReaction(reaction string) reactions.EmojiID {
	switch reaction {
	default:
		return reactions.EmojiID(reaction)
	case "laugh":
		return "smile"
	case "hooray":
		return "tada"
	}
}

// externalizeReaction converts reactions.EmojiID to github.Reaction.Content.
func externalizeReaction(reaction reactions.EmojiID) string {
	switch reaction {
	default:
		return string(reaction)
	case "smile":
		return "laugh"
	case "tada":
		return "hooray"
	}
}
