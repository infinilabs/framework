---
title: "File Read Guard"
weight: 17
---

# File Read Guard

`util.ReadGuard` restricts file reads to allowed directories in applications that use INFINI Framework.
It accepts literal directories and glob patterns, which select directories by name.
The API is in `infini.sh/framework/core/util`.

## Usage and API

Construct a guard with the directories or patterns that the application permits.
Use actual paths for access requests, without glob syntax or escaping.
`ResolveUnder` returns a resolved path. It does not open the file.

```go
guard, err := util.NewReadGuard("/srv/easysearch/node-*/logs")
if err != nil {
    return err
}

file, err := guard.ResolveUnder("/srv/easysearch/node-a/logs", "server.log")
if err != nil {
    return err
}
content, err := os.ReadFile(file)
```

| Function or method | Behavior |
| --- | --- |
| `NewReadGuard(roots ...string)` | Cleans and validates every rule. An invalid rule fails construction. Empty entries are ignored, but an empty rule set fails. |
| `Roots()` | Returns a copy of root descriptions. Literal roots are resolved absolute paths. Patterns retain their configuration spelling, with native separators. |
| `Contains(path)` | Reports whether the resolved actual path matches an allowed root itself. |
| `ContainsUnder(path)` | Reports whether the resolved actual path matches an allowed root or any descendant. |
| `ResolveUnder(base, file)` | Requires an allowed base and an existing regular file strictly below an allowed root. The file name can be relative or absolute. |

`Contains` and `ContainsUnder` test authorization without requiring the requested path to exist.
`ResolveUnder` also requires the target to exist and be a regular file, rather than a directory or device.
Multiple rules form a union. Rule order does not change access.
A relative file name can leave `base`, provided that its resolved target stays strictly below an allowed root.

For the rule `/srv/easysearch/node-*/logs`:

| Actual path | `Contains` | `ContainsUnder` |
| --- | --- | --- |
| `/srv/easysearch/node-a/logs` | `true` | `true` |
| `/srv/easysearch/node-a/logs/gc/archive` | `false` | `true` |
| `/srv/easysearch/node-a/extra/logs` | `false` | `false` |
| `/srv/easysearch/node-a/logs-backup` | `false` | `false` |
| `/srv/easysearch/node-a/config` | `false` | `false` |

## Glob syntax

All platforms use [gobwas/glob](https://github.com/gobwas/glob) with `/` as the matching separator.
Patterns are compiled during construction and matched when access is requested.
The guard does not enumerate matching directories, so new matching directories work with the same guard.

| Syntax | Meaning | Example |
| --- | --- | --- |
| `*` | Zero or more characters within one path component | `node-*/logs` |
| `?` | One character within one path component | `node-?/logs` |
| `**` | Zero or more characters, including `/` | `node-**/logs` |
| `[abc]` | One character from a set | `node-[ab]/logs` |
| `[a-z]` | One character from a range | `node-[a-c]/logs` |
| `[!abc]` | One character outside a set | `node-[!ab]/logs` |
| `{a,b}` | Either alternative, including nested patterns | `{node-a,node-b}/logs` |

The separator restriction applies to `*` and `?`.
Character groups retain the library behavior: `[!a]` can match `/`.
Alternatives can contain directory separators, as in `{group/node-a,node-b}/logs`.

A single-component pattern selects an allowed root, but access below that root remains recursive.
For example, `node-*/logs` allows `node-a/logs/gc/archive/server.log`.
It does not select `node-a/extra/logs` as a root. Use `node-**/logs` to select that deeper root.

## Relative paths and cleanup

Relative rules use the process working directory, rather than the configuration file directory.
ReadGuard does not expand `~` or environment variables.
Any expansion performed by an application before it calls ReadGuard belongs to that application.

Cleanup removes `.` components, duplicate separators, and trailing directory separators.
A `..` component removes the preceding expression component, including any wildcard fragments in it.
Cleanup does not require the removed component to exist and cannot move above the filesystem root.

For a working directory of `/srv/agent`:

| Input rule | Cleaned rule |
| --- | --- |
| `./data/node-*/logs` | `/srv/agent/data/node-*/logs` |
| `./data//node-*/./logs/` | `/srv/agent/data/node-*/logs` |
| `/srv/node-*/../logs` | `/srv/logs` |
| `/srv/**/../logs` | `/srv/logs` |

This cleanup acts on expression components, before matching actual paths.
It removes `**` as one expression component before `..`, regardless of the number of directories that `**` can match.
Each alternative uses its own component stack, so different directory depths remain distinct:

```text
/srv/easysearch/{a/b,c}/../logs

/srv/easysearch/a/b/../logs → /srv/easysearch/a/logs
/srv/easysearch/c/../logs   → /srv/easysearch/logs
```

## Escaping and platform paths

Unescaped glob characters in rules have pattern meanings, even if a directory with those literal characters exists.
Existing literal rules that contain glob characters need the escaping described below.
Request paths remain actual paths and do not use these escapes.

On Unix, backslash escapes the next character.
Use `\*`, `\?`, `\[`, and `\{` for literal metacharacters, and `\\` for a literal backslash.
For example, `/srv/easysearch/node\*/logs` selects the actual directory named `node*`, rather than directories such as `node-a`.

On Windows, both `/` and `\` are path separators, including mixed usage.
Backslash escaping is disabled: `C:\easysearch\*\logs` and `C:/easysearch/*/logs` are equivalent patterns.
UNC network paths such as `\\server\share\easysearch\*\logs` retain their share root.
Use singleton character groups for literal metacharacters that Windows file names allow, such as `[[]` for `[` and `[{]` for `{`.
For example, `C:/easysearch/[[]node]/logs` selects the directory named `[node]`.
The guard does not add case folding.

Agent passes `elasticsearch_logs.allowed_paths` to ReadGuard.
YAML single quotes preserve backslashes, so these examples avoid an extra YAML escaping layer:

```yaml
elasticsearch_logs:
  allowed_paths:
    - './data/agent/nodes/*/services/easysearch/*/assets/easysearch-*/logs'
```

For a literal Unix star, use `'/srv/easysearch/node\*/logs'`.
The equivalent YAML double-quoted value is `"/srv/easysearch/node\\*/logs"`.
On Windows, `'C:\easysearch\*\logs'` and `'C:/easysearch/*/logs'` both select matching log directories.

## Directory validation and symbolic links

A literal root must exist, be a directory, and pass the guard's system-path and scope restrictions.
For a pattern, its fixed directory prefix must meet the same requirements.
The fixed prefix is the common sequence of literal directory components before any uncertainty in the cleaned rule.
The selected directories below that prefix do not need to exist at construction.

For example, `/var/log/easysearch/*/logs` requires `/var/log/easysearch` to exist.
`/home/*/logs` fails because `/home` is too broad as a fixed prefix.
System locations such as `/etc` and `/usr/bin`, the Unix root, and Windows drive roots are rejected.
Cleanup cannot bypass these restrictions: `/srv/node-*/..` becomes `/srv` and fails the scope restriction.

Literal roots resolve symbolic links during construction.
Patterns preserve their cleaned expression paths and do not map symbolic-link aliases to their targets.
Every actual request path resolves symbolic links before matching, including the final file in `ResolveUnder`.
On Windows, literal roots and actual request paths also resolve directory junctions through Windows file handles.
For example, a request through `assets/easysearch/logs` can match `assets/easysearch-*/logs` when the junction targets `assets/easysearch-2.5.0-windows-amd64`.
Junctions inside a permitted log directory cannot authorize targets outside the allowed roots.

Read and deletion guards share path canonicalization and follow both symbolic links and Windows junctions.
`SafeRemoveAll` removes the resolved target, which must pass the dangerous-path and allowed-root checks.

For example, suppose `/srv/current` links to `/srv/easysearch`.
The pattern `/srv/current/node-*/logs` does not authorize a target that resolves to `/srv/easysearch/node-a/logs`.
Use the target path in a pattern, or a literal root such as `/srv/current/node-a/logs`.
A link that resolves outside all allowed roots fails authorization.
A separate explicit rule can authorize that target through the same union of rules.

`Roots()` is a description API, rather than a configuration serializer.
Its results can contain literal metacharacters, so apply platform-specific escaping before using those results as new rule inputs.
