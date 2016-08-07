package githubapi

import (
	"github.com/shurcooL/issues"
	"github.com/shurcooL/notifications"
	"golang.org/x/net/context"
)

// markRead marks the specified issue as read for current user.
func (s service) markRead(ctx context.Context, repo issues.RepoSpec, id uint64) error {
	if s.notifications == nil {
		return nil
	}

	return s.notifications.MarkRead(ctx, "Issue", notifications.RepoSpec{URI: repo.URI}, id)
}
