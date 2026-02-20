# go-mod-archived

Detect archived GitHub dependencies in Go projects.

Parses your `go.mod`, queries the GitHub GraphQL API in batches, and reports which dependencies have been archived upstream.

## Install

```bash
go install github.com/norman-abramovitz/go-mod-archived@latest
```

Or build from source:

```bash
git clone https://github.com/norman-abramovitz/go-mod-archived.git
cd go-mod-archived
go build -o go-mod-archived .
```

## Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`) installed and authenticated — used to obtain your GitHub API token
- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) — required only for `--files` flag

## Usage

```
go-mod-archived [flags] [path/to/go.mod | path/to/dir]
```

If no path is given, looks for `go.mod` in the current directory. You can also pass a directory path and the tool will look for `go.mod` inside it. Flags can appear before or after the path.

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--all` | Show all modules, not just archived ones |
| `--direct-only` | Only check direct dependencies (skip indirect) |
| `--tree` | Show dependency tree for archived modules (uses `go mod graph`) |
| `--files` | Show source files that import archived modules (requires `rg`) |
| `--time` | Include time in date output (2006-01-02 15:04:05 instead of 2006-01-02) |
| `--workers N` | Repos per GitHub GraphQL batch request (default 50) |

### Exit codes

- `0` — no archived dependencies found
- `1` — archived dependencies detected (useful in CI)
- `2` — error (bad path, parse failure, API error)

## Examples

### Default table output

```
$ go-mod-archived
Checking 234 GitHub modules...

ARCHIVED DEPENDENCIES (19 of 234 github.com modules)

MODULE                                     VERSION   DIRECT    ARCHIVED AT  LAST PUSHED
github.com/mitchellh/copystructure         v1.2.0    direct    2024-07-22   2021-05-05
github.com/mitchellh/mapstructure          v1.5.0    indirect  2024-07-22   2024-06-25
github.com/pkg/errors                      v0.9.1    indirect  2021-12-01   2021-11-02
...

Skipped 61 non-GitHub modules.
```

### Direct dependencies only

```
$ go-mod-archived --direct-only
Checking 83 GitHub modules...

ARCHIVED DEPENDENCIES (5 of 83 github.com modules)

MODULE                                     VERSION   DIRECT  ARCHIVED AT  LAST PUSHED
github.com/google/go-metrics-stackdriver   v0.2.0    direct  2024-12-03   2023-09-29
github.com/mitchellh/copystructure         v1.2.0    direct  2024-07-22   2021-05-05
github.com/mitchellh/go-testing-interface  v1.14.2   direct  2023-10-31   2021-08-21
github.com/mitchellh/pointerstructure      v1.2.1    direct  2024-07-22   2023-09-06
github.com/mitchellh/reflectwalk           v1.0.2    direct  2024-07-22   2022-04-21
```

### Dependency tree

Shows which direct dependencies transitively pull in archived modules:

```
$ go-mod-archived --tree
github.com/Masterminds/sprig/v3
  ├── github.com/mitchellh/copystructure [ARCHIVED]
  └── github.com/mitchellh/reflectwalk [ARCHIVED]
github.com/hashicorp/go-discover
  ├── github.com/Azure/go-autorest/autorest [ARCHIVED]
  ├── github.com/aws/aws-sdk-go [ARCHIVED]
  ├── github.com/denverdino/aliyungo [ARCHIVED]
  ├── github.com/nicolai86/scaleway-sdk [ARCHIVED]
  └── github.com/pkg/errors [ARCHIVED]
github.com/mitchellh/copystructure [ARCHIVED]
  └── github.com/mitchellh/reflectwalk [ARCHIVED]
```

### Source file scanning

Shows which source files import each archived module, helping prioritize replacements:

```
$ go-mod-archived --files
...
SOURCE FILES IMPORTING ARCHIVED MODULES

github.com/mitchellh/copystructure (10 files)
  audit/hashstructure.go:14
  sdk/logical/request.go:14
  vault/acl.go:21
  vault/mount_entry.go:14
  vault/policy.go:17
  ...

github.com/mitchellh/reflectwalk (1 file)
  audit/hashstructure.go:15

github.com/pkg/errors (0 files)
```

Combines with `--json` to add `source_files` arrays, or with `--tree` to append file counts:

```
$ go-mod-archived --files --tree
github.com/Masterminds/sprig/v3@v3.2.3
  ├── github.com/mitchellh/copystructure@v1.2.0 [ARCHIVED 2024-07-22] (10 files)
  └── github.com/mitchellh/reflectwalk@v1.0.2 [ARCHIVED 2024-07-22] (1 file)
```

### JSON output

```
$ go-mod-archived --json
{
  "archived": [
    {
      "module": "github.com/mitchellh/copystructure",
      "version": "v1.2.0",
      "direct": true,
      "owner": "mitchellh",
      "repo": "copystructure",
      "archived_at": "2024-07-22T20:44:18Z",
      "pushed_at": "2021-05-05T17:08:29Z"
    }
  ],
  "skipped_non_github": 61,
  "total_checked": 234
}
```

### JSON dependency tree

Combine `--tree --json` for a structured tree showing which direct dependencies pull in archived transitive deps:

```
$ go-mod-archived --tree --json
{
  "tree": [
    {
      "module": "github.com/Masterminds/sprig/v3",
      "version": "v3.2.3",
      "archived": false,
      "archived_dependencies": [
        {
          "module": "github.com/mitchellh/copystructure",
          "version": "v1.2.0",
          "archived_at": "2024-07-22T20:44:18Z",
          "pushed_at": "2021-05-05T17:08:29Z"
        },
        {
          "module": "github.com/mitchellh/reflectwalk",
          "version": "v1.0.2",
          "archived_at": "2024-07-22T20:48:05Z",
          "pushed_at": "2022-04-21T16:48:49Z"
        }
      ]
    },
    {
      "module": "github.com/mitchellh/copystructure",
      "version": "v1.2.0",
      "archived": true,
      "archived_at": "2024-07-22T20:44:18Z",
      "pushed_at": "2021-05-05T17:08:29Z",
      "archived_dependencies": []
    }
  ],
  "skipped_non_github": 61,
  "total_checked": 234
}
```

Add `--files` to include `source_files` arrays on archived entries.

## How it works

1. Parses `go.mod` using `golang.org/x/mod/modfile`
2. Extracts `owner/repo` from `github.com/*` module paths, deduplicating multi-path repos (e.g., `github.com/foo/bar/v2` and `github.com/foo/bar/sdk/v2`)
3. Batches repos into GitHub GraphQL queries (~50 per request) checking `isArchived`, `archivedAt`, and `pushedAt`
4. Non-GitHub modules (`golang.org/x/*`, `google.golang.org/*`, `gopkg.in/*`, etc.) are skipped with a summary count

## License

MIT
