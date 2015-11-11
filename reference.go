package issues

import (
	"html/template"
)

// Reference represents a reference to code.
type Reference struct {
	Repo RepoSpec
	// Path is a relative, /-separated path to a file within a repo.
	Path      string
	CommitID  string
	StartLine int
	EndLine   int
	Contents  template.HTML
}
