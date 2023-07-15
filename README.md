issues
======

[![Go Reference](https://pkg.go.dev/badge/github.com/shurcooL/issues.svg)](https://pkg.go.dev/github.com/shurcooL/issues)

Package issues provides an issues service definition.

Installation
------------

```sh
go get github.com/shurcooL/issues
```

Directories
-----------

| Path                                                                 | Synopsis                                                                                |
|----------------------------------------------------------------------|-----------------------------------------------------------------------------------------|
| [fs](https://pkg.go.dev/github.com/shurcooL/issues/fs)               | Package fs implements issues.Service using a virtual filesystem.                        |
| [githubapi](https://pkg.go.dev/github.com/shurcooL/issues/githubapi) | Package githubapi implements issues.Service using GitHub API clients.                   |
| [maintner](https://pkg.go.dev/github.com/shurcooL/issues/maintner)   | Package maintner implements a read-only issues.Service using a x/build/maintner corpus. |

License
-------

-	[MIT License](LICENSE)
