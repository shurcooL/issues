issues
======

[![Build Status](https://travis-ci.org/shurcooL/issues.svg?branch=master)](https://travis-ci.org/shurcooL/issues) [![GoDoc](https://godoc.org/github.com/shurcooL/issues?status.svg)](https://godoc.org/github.com/shurcooL/issues)

Package issues provides an issues service definition.

Installation
------------

```bash
go get -u github.com/shurcooL/issues
```

Directories
-----------

| Path                                                                    | Synopsis                                                                          |
|-------------------------------------------------------------------------|-----------------------------------------------------------------------------------|
| [asanaapi](https://godoc.org/github.com/shurcooL/issues/asanaapi)       | Package asanaapi implements issues.Service using Asana API client.                |
| [fs](https://godoc.org/github.com/shurcooL/issues/fs)                   | Package fs implements issues.Service using a filesystem.                          |
| [githubapi](https://godoc.org/github.com/shurcooL/issues/githubapi)     | Package githubapi implements issues.Service using GitHub API client.              |
| [githubqlapi](https://godoc.org/github.com/shurcooL/issues/githubqlapi) | Package githubqlapi implements issues.Service using GitHub GraphQL API v4 client. |

License
-------

-	[MIT License](https://opensource.org/licenses/mit-license.php)
