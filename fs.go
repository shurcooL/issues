// Package fs implements issues.Service using a filesystem.
package fs

import "src.sourcegraph.com/sourcegraph/platform/apps/issues2/issues"

func NewService() issues.Service {
	return service{}
}

// TODO.
type service struct{ issues.Service }
