package githubapi

import (
	"github.com/google/go-github/github"
	"github.com/shurcooL/reactions"
	"github.com/shurcooL/users"
	"golang.org/x/net/context"
)

// reactions converts a []*github.Reaction to []reactions.Reaction.
// It makes use of users service to get user details.
func (s service) reactions(ctx context.Context, ghReactions []*github.Reaction) ([]reactions.Reaction, error) {
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

		// Only get the details of first few users and authed user.
		const expandUsers = 10
		isAuthedUser := s.currentUser.ID != 0 && uint64(*reaction.UserID) == s.currentUser.ID
		if len(rs[i].Users) < expandUsers || isAuthedUser {
			user, err := s.users.Get(ctx, users.UserSpec{ID: uint64(*reaction.UserID), Domain: "github.com"})
			if err != nil {
				return rs, err
			}
			rs[i].Users = append(rs[i].Users, user)
		} else {
			rs[i].Users = append(rs[i].Users, users.User{})
		}
	}
	return rs, nil
}

// listIssueReactions fetches all pages.
func (s service) listIssueReactions(owner, repo string, id int) ([]*github.Reaction, error) {
	var issueReactions []*github.Reaction
	ghOpt := &github.ListOptions{}
	for {
		irs, resp, err := s.cl.Reactions.ListIssueReactions(owner, repo, id, ghOpt)
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
func (s service) listIssueCommentReactions(owner, repo string, id int) ([]*github.Reaction, error) {
	var commentReactions []*github.Reaction
	ghOpt := &github.ListOptions{}
	for {
		crs, resp, err := s.cl.Reactions.ListIssueCommentReactions(owner, repo, id, ghOpt)
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
