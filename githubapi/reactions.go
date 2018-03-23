package githubapi

import (
	"fmt"

	"github.com/shurcooL/githubql"
	"github.com/shurcooL/reactions"
	"github.com/shurcooL/users"
)

type reactionGroups []struct {
	Content githubql.ReactionContent
	Users   struct {
		Nodes      []*githubqlUser
		TotalCount int
	} `graphql:"users(first:10)"`
	ViewerHasReacted bool
}

// ghReactions converts []githubql.ReactionGroup to []reactions.Reaction.
func ghReactions(rgs reactionGroups, viewer users.User) []reactions.Reaction {
	var rs []reactions.Reaction
	for _, rg := range rgs {
		if rg.Users.TotalCount == 0 {
			continue
		}

		// Only return the details of first few users and viewer.
		var us []users.User
		addedViewer := false
		for i := 0; i < rg.Users.TotalCount; i++ {
			if i < len(rg.Users.Nodes) {
				user := ghUser(rg.Users.Nodes[i])
				us = append(us, user)
				if user.UserSpec == viewer.UserSpec {
					addedViewer = true
				}
			} else if i == len(rg.Users.Nodes) {
				// Add viewer last if they've reacted, but haven't been added already.
				if rg.ViewerHasReacted && !addedViewer {
					us = append(us, viewer)
				} else {
					us = append(us, users.User{})
				}
			} else {
				us = append(us, users.User{})
			}
		}

		rs = append(rs, reactions.Reaction{
			Reaction: internalizeReaction(rg.Content),
			Users:    us,
		})
	}
	return rs
}

// internalizeReaction converts githubql.ReactionContent to reactions.EmojiID.
func internalizeReaction(reaction githubql.ReactionContent) reactions.EmojiID {
	switch reaction {
	case githubql.ReactionContentThumbsUp:
		return "+1"
	case githubql.ReactionContentThumbsDown:
		return "-1"
	case githubql.ReactionContentLaugh:
		return "smile"
	case githubql.ReactionContentHooray:
		return "tada"
	case githubql.ReactionContentConfused:
		return "confused"
	case githubql.ReactionContentHeart:
		return "heart"
	default:
		panic("unreachable")
	}
}

// externalizeReaction converts reactions.EmojiID to githubql.ReactionContent.
func externalizeReaction(reaction reactions.EmojiID) (githubql.ReactionContent, error) {
	switch reaction {
	case "+1":
		return githubql.ReactionContentThumbsUp, nil
	case "-1":
		return githubql.ReactionContentThumbsDown, nil
	case "smile":
		return githubql.ReactionContentLaugh, nil
	case "tada":
		return githubql.ReactionContentHooray, nil
	case "confused":
		return githubql.ReactionContentConfused, nil
	case "heart":
		return githubql.ReactionContentHeart, nil
	default:
		return "", fmt.Errorf("%q is an unsupported reaction", reaction)
	}
}
