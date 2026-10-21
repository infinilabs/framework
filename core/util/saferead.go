/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gobwas/glob"
	"github.com/gobwas/glob/compiler"
	"github.com/gobwas/glob/syntax"
	"github.com/gobwas/glob/syntax/ast"
)

// Read-side access control confines externally requested reads to allowed directories.
//
// Caller-supplied paths must not turn a log endpoint into a general-purpose file
// reader. System locations remain forbidden even when configured as allowed roots.
//
// IsSystemReadPath rejects system locations. ReadGuard requires files to be strict
// descendants of literal or glob roots. Literal roots and actual request paths
// resolve symlinks. Glob rules do not resolve symlink aliases.

// systemReadPrefixes lists trees that never legitimately hold readable
// service logs, so serving reads from them is refused outright. It is
// deliberately narrower than safedelete's neverTouch: packaged products
// keep logs under /usr/share/<product>/logs (RPM/DEB installs) and
// /var/log, /opt, /home are standard log locations — those stay
// reachable, constrained by the whitelist instead.
var systemReadPrefixes = []string{
	"/bin", "/sbin", "/boot", "/dev", "/etc", "/proc", "/sys", "/run",
	"/lib", "/lib64", "/libx32", "/root", "/kernel",
	"/System", "/private/etc",
	"/usr/bin", "/usr/sbin", "/usr/lib", "/usr/lib64", "/usr/libx32",
}

// windowsSystemReadPrefixes are matched against the volume-relative part
// of a drive path (C:/Windows/... -> /Windows/...) case-insensitively.
var windowsSystemReadPrefixes = []string{
	"/Windows", "/Program Files/WindowsApps",
}

// tooBroadReadRoots lists top-level trees that hold far more than logs;
// whitelisting one of them verbatim (instead of a specific subdirectory)
// is treated as a misconfiguration and rejected. Their contents remain
// reachable through narrower roots.
var tooBroadReadRoots = []string{
	"/usr", "/var", "/opt", "/srv", "/home", "/mnt", "/media", "/tmp",
	"/private/var", "/private/tmp",
}

// IsSystemReadPath reports whether p is an OS-critical location that must
// never be served by a file-reading API: the filesystem root, a
// systemReadPrefixes member or anything under it (drive roots and the
// listed Windows trees on Windows).
func IsSystemReadPath(p string) bool {
	return isSystemReadCanonical(canonicalReadPath(p))
}

// canonicalReadPath canonicalizes p and maps macOS firmlink paths
// (/System/Volumes/Data/...) back to their firmlink view (/home, /opt,
// ...) so every guard layer compares the same spelling.
func canonicalReadPath(p string) string {
	c := canonicalPath(p)
	if strings.HasPrefix(c, "/System/Volumes/Data/") {
		c = c[len("/System/Volumes/Data"):]
	}
	return c
}

// isSystemReadCanonical applies the deny checks to an already-canonical
// path so the Windows rules stay testable on every platform.
func isSystemReadCanonical(c string) bool {
	if c == "/" || c == string(filepath.Separator) {
		return true
	}
	// drive-qualified path (Windows): compare the volume-relative part.
	// The separator check keeps unix paths whose second character happens
	// to be ':' (e.g. /t:mp) on the unix branch.
	if len(c) > 2 && c[1] == ':' && (c[2] == '/' || c[2] == '\\') {
		c = filepath.ToSlash(c)
		if len(c) <= 3 { // bare drive root, e.g. C:/
			return true
		}
		rel := strings.ToLower(c[2:]) // /windows/system32
		for _, prefix := range windowsSystemReadPrefixes {
			prefix = strings.ToLower(prefix)
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
		}
		return false
	}
	for _, prefix := range systemReadPrefixes {
		if c == prefix || strings.HasPrefix(c, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// isTooBroadReadRoot reports whether an already-canonical directory is a
// wholesale top-level tree rather than a specific log location.
func isTooBroadReadRoot(c string) bool {
	for _, broad := range tooBroadReadRoots {
		if c == broad {
			return true
		}
	}
	return false
}

// readPathComponent keeps all AST fragments belonging to one path component.
//
// A name such as node-* must be removed as one component by dot-dot, rather than
// treating its literal and wildcard fragments as separate directory levels.
type readPathComponent struct {
	nodes []*ast.Node
}

// Helper function to identify components with a single literal meaning.
//
// Lexical cleanup needs to recognize dot components, and root validation needs
// a fixed directory prefix. Neither operation can treat wildcard nodes as text.
func (c readPathComponent) literal() (string, bool) {
	var text strings.Builder
	for _, node := range c.nodes {
		value, ok := readPatternLiteral(node)
		if !ok {
			return "", false
		}
		text.WriteString(value)
	}
	return text.String(), true
}

// Helper function to recover deterministic text from parsed glob nodes.
//
// Windows uses backslashes as separators, so users need singleton character
// classes to spell literal metacharacters. These classes must also participate
// in literal-path recognition and fixed-prefix validation.
//
// A slash class stays a glob node. Treating it as text would let component
// cleanup collapse it with an adjacent separator and change the matching set.
func readPatternLiteral(node *ast.Node) (string, bool) {
	switch node.Kind {
	case ast.KindText:
		return node.Value.(ast.Text).Text, true
	case ast.KindList:
		list := node.Value.(ast.List)
		if !list.Not && list.Chars != "/" && utf8.RuneCountInString(list.Chars) == 1 {
			return list.Chars, true
		}
	}
	return "", false
}

// readPathState tracks the root, completed components, and unfinished component
// of a parsed path expression.
//
// Alternatives can have different directory depths or supply parts of a file
// name. Each branch needs an independent state before consuming the common suffix.
type readPathState struct {
	anchor     string
	components []readPathComponent
	current    readPathComponent
}

// Helper function to complete a component at a separator or the end of a rule.
//
// Literal and wildcard names need the same dot-component rules. Removing a
// wildcard component is a lexical operation and does not require matching directories.
//
// The state must already have an absolute anchor, which dot-dot cannot remove.
func (s *readPathState) finishComponent() {
	literal, known := s.current.literal()
	if known && literal == ".." {
		if len(s.components) > 0 {
			s.components = s.components[:len(s.components)-1]
		}
	} else if !known || (literal != "" && literal != ".") {
		s.components = append(s.components, s.current)
	}
	s.current = readPathComponent{}
}

// Helper function to append parsed text at explicit path boundaries.
//
// The initial literal prefix and later AST text need identical cleanup. Sharing
// this step prevents dot components and repeated separators from behaving
// differently before and after a wildcard.
func (s *readPathState) appendText(text string) {
	parts := strings.Split(text, "/")
	for i, part := range parts {
		if part != "" {
			s.current.nodes = append(s.current.nodes, ast.NewNode(ast.KindText, ast.Text{Text: part}))
		}
		if i+1 < len(parts) {
			s.finishComponent()
		}
	}
}

// Helper function to set the absolute root and clean the buffered literal prefix.
//
// An alternative can complete a drive prefix, as in C:{/a,/b}. Anchoring C:
// before reading the branch would incorrectly add that drive's working directory.
//
// Only literal text is buffered before this step. Relative prefixes use the
// process working directory, while drive and UNC roots keep their native meaning.
func (s *readPathState) startPath() error {
	if s.anchor != "" {
		return nil
	}
	text, _ := s.current.literal()
	volume := filepath.VolumeName(filepath.FromSlash(text))
	tail := text[len(volume):]
	base := volume + "."
	if strings.HasPrefix(tail, "/") {
		base = volume + string(filepath.Separator)
	}
	absolute, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	baseVolume := filepath.VolumeName(absolute)
	s.anchor = filepath.ToSlash(baseVolume) + "/"
	remaining := strings.TrimLeft(filepath.ToSlash(absolute[len(baseVolume):]), "/")
	if remaining != "" {
		for _, part := range strings.Split(remaining, "/") {
			s.components = append(s.components, readPathComponent{nodes: []*ast.Node{
				ast.NewNode(ast.KindText, ast.Text{Text: part}),
			}})
		}
	}
	s.current = readPathComponent{}
	s.appendText(strings.TrimLeft(tail, "/"))
	return nil
}

// consume cleans parsed path expressions into independent branch states.
//
// Alternatives can contain different numbers of directory components. Each branch
// needs its own stack so a following dot-dot removes the correct component.
//
// Branch boundaries do not imply path separators. The final component stays open
// until the whole rule ends, and glob syntax remains in library AST nodes.
func (s readPathState) consume(nodes []*ast.Node) ([]readPathState, error) {
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		if node.Kind == ast.KindAnyOf {
			var results []readPathState
			for _, branch := range node.Children {
				next := s
				next.components = append([]readPathComponent(nil), s.components...)
				next.current.nodes = append([]*ast.Node(nil), s.current.nodes...)
				states, err := next.consume(branch.Children)
				if err != nil {
					return nil, err
				}
				for _, state := range states {
					continued, err := state.consume(nodes[i+1:])
					if err != nil {
						return nil, err
					}
					results = append(results, continued...)
				}
			}
			return results, nil
		}

		text, literal := readPatternLiteral(node)
		if literal && text == "" {
			continue
		}

		if s.anchor == "" {
			if literal {
				s.current.nodes = append(s.current.nodes, ast.NewNode(ast.KindText, ast.Text{Text: text}))
				continue
			}
			if err := s.startPath(); err != nil {
				return nil, err
			}
		}

		if !literal {
			s.current.nodes = append(s.current.nodes, node)
			continue
		}
		s.appendText(text)
	}
	return []readPathState{s}, nil
}

// readRoot stores a canonical literal directory or a compiled glob root rule.
//
// Matching directories can appear after construction. Compiled rules authorize
// them without enumerating the filesystem or parsing patterns for each request.
//
// Literal roots leave both matchers nil. Published matchers are immutable.
type readRoot struct {
	path        string
	pattern     glob.Glob
	descendants glob.Glob
}

// Helper function to match an already-canonical path against a root itself.
//
// Exact-root queries and inclusive descendant queries need the same root test.
// Request characters remain literal path characters, rather than new glob syntax.
func (r readRoot) matches(path string) bool {
	if r.pattern != nil {
		return r.pattern.Match(path)
	}
	return path == r.path
}

// Helper function to match descendants, optionally including the root itself.
//
// Directory-list requests can name the root itself, while file reads require a
// strict descendant. Sharing this check preserves that boundary for both rule types.
func (r readRoot) contains(path string, includeRoot bool) bool {
	if includeRoot && r.matches(path) {
		return true
	}
	if r.descendants != nil {
		return r.descendants.Match(path)
	}
	return strings.HasPrefix(path, r.path+"/")
}

// Helper function to copy a parsed rule for independent root and descendant compilation.
//
// The library compiler mutates its input tree during optimization. Independent
// children and parent links prevent one compilation from changing another rule.
func cloneReadPattern(node *ast.Node) *ast.Node {
	copied := ast.NewNode(node.Kind, node.Value)
	for _, child := range node.Children {
		ast.Insert(copied, cloneReadPattern(child))
	}
	return copied
}

// ReadGuard confines file reads to literal roots and glob root rules.
//
// Services can create node-specific log directories after guard construction.
// Literal roots and compiled patterns allow those reads without scanning for matches.
//
// Actual request paths resolve symlinks before matching. Literal roots also resolve
// links during construction, while glob rules preserve their expression paths.
type ReadGuard struct {
	roots []readRoot
}

// NewReadGuard cleans and validates literal roots and glob rules.
//
// Future matching directories cannot be checked at construction. Validating each
// pattern's common fixed directory retains the scope restrictions used for literal roots.
//
// Literal roots and fixed directories must exist. Patterns use gobwas/glob syntax
// with '/' as the separator. Windows accepts both separators and treats backslashes
// as separators rather than escapes.
func NewReadGuard(roots ...string) (*ReadGuard, error) {
	g := &ReadGuard{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		// Normalize both Windows path separators to '/'.
		//
		// Windows accepts both forward slashes and backslashes. Using one separator
		// simplifies our manual path component splitting. On Unix, backslashes
		// remain glob escapes.
		root = filepath.ToSlash(root)
		tree, err := syntax.Parse(root)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed read pattern %q: %w", root, err)
		}
		states, err := (readPathState{}).consume(tree.Children)
		if err != nil {
			return nil, err
		}
		var common []string
		literal := true
		for i := range states {
			state := &states[i]
			if err := state.startPath(); err != nil {
				return nil, err
			}
			state.finishComponent()
			var prefix []string
			for _, component := range state.components {
				text, ok := component.literal()
				if !ok {
					break
				}
				prefix = append(prefix, text)
			}
			if i == 0 {
				common = prefix
			} else {
				if state.anchor != states[0].anchor {
					return nil, fmt.Errorf("allowed pattern %q has no common directory root", root)
				}
				n := 0
				for n < len(common) && n < len(prefix) && common[n] == prefix[n] {
					n++
				}
				common = common[:n]
			}
		}
		for _, state := range states {
			if len(common) != len(state.components) {
				literal = false
			}
		}
		fixed := states[0].anchor + strings.Join(common, "/")
		c := canonicalReadPath(filepath.FromSlash(fixed))
		if IsSystemReadPath(c) {
			return nil, fmt.Errorf("refusing to serve reads from system path: %s", c)
		}
		if isTooBroadReadRoot(c) {
			return nil, fmt.Errorf("allowed path %s is too broad, whitelist a specific log directory instead", c)
		}
		fi, err := os.Stat(c)
		if err != nil {
			return nil, fmt.Errorf("allowed path %s is not accessible: %v", c, err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("allowed path %s is not a directory", c)
		}
		if literal {
			g.roots = append(g.roots, readRoot{path: filepath.ToSlash(c)})
			continue
		}

		tree = ast.NewNode(ast.KindPattern, nil)
		alternatives := ast.NewNode(ast.KindAnyOf, nil)
		for _, state := range states {
			branch := ast.NewNode(ast.KindPattern, nil,
				ast.NewNode(ast.KindText, ast.Text{Text: state.anchor}))
			for i, component := range state.components {
				if i > 0 {
					ast.Insert(branch, ast.NewNode(ast.KindText, ast.Text{Text: "/"}))
				}
				for _, node := range component.nodes {
					ast.Insert(branch, cloneReadPattern(node))
				}
			}
			ast.Insert(alternatives, branch)
		}
		if len(alternatives.Children) == 1 {
			tree = alternatives.Children[0]
			tree.Parent = nil
		} else {
			ast.Insert(tree, alternatives)
		}
		rootTree := cloneReadPattern(tree)
		descendantTree := cloneReadPattern(tree)
		ast.Insert(descendantTree,
			ast.NewNode(ast.KindText, ast.Text{Text: "/"}),
			ast.NewNode(ast.KindSuper, nil))
		pattern, err := compiler.Compile(rootTree, []rune{'/'})
		if err != nil {
			return nil, fmt.Errorf("compile allowed read pattern %q: %w", root, err)
		}
		descendants, err := compiler.Compile(descendantTree, []rune{'/'})
		if err != nil {
			return nil, fmt.Errorf("compile allowed read descendants %q: %w", root, err)
		}
		g.roots = append(g.roots, readRoot{path: root, pattern: pattern, descendants: descendants})
	}
	if len(g.roots) == 0 {
		return nil, fmt.Errorf("no allowed read paths configured")
	}
	return g, nil
}

// Roots returns a copy of root descriptions in native path syntax.
//
// Callers need directory and rule descriptions without mutable access to the guard.
//
// Literal roots are canonical paths. Patterns retain their configuration spelling.
// The descriptions are not escaped for reuse as configuration inputs.
func (g *ReadGuard) Roots() []string {
	roots := make([]string, len(g.roots))
	for i, root := range g.roots {
		roots[i] = filepath.FromSlash(root.path)
	}
	return roots
}

// Contains reports whether the resolved path matches an allowed root itself.
//
// Callers that distinguish roots from nested directories need an exact-root check.
// This checks authorization without requiring the requested path to exist.
func (g *ReadGuard) Contains(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	// Match using the same '/' separators as the compiled roots.
	//
	// Windows accepts both forward slashes and backslashes. Root parsing unifies
	// them to simplify our manual path component splitting, so request paths
	// must use the same separator when matching.
	c := filepath.ToSlash(canonicalReadPath(path))
	for _, r := range g.roots {
		if r.matches(c) {
			return true
		}
	}
	return false
}

// ContainsUnder reports whether the resolved path matches an allowed root or descendant.
//
// Log viewers can request nested directories, such as a GC log directory below
// path.logs. Requiring an exact-root match would reject these authorized requests.
//
// This checks authorization without requiring the requested path to exist.
func (g *ReadGuard) ContainsUnder(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	// Match using the same '/' separators as the compiled roots.
	//
	// Windows accepts both forward slashes and backslashes. Root parsing unifies
	// them to simplify our manual path component splitting, so request paths
	// must use the same separator when matching.
	c := filepath.ToSlash(canonicalReadPath(path))
	for _, r := range g.roots {
		if r.contains(c, true) {
			return true
		}
	}
	return false
}

// ResolveUnder returns an existing regular file strictly below an allowed root.
//
// A permitted base does not guarantee that a supplied file stays inside an allowed
// tree. The resolved target needs its own authorization and file-type check.
//
// File names can be relative or absolute. They can leave base if the resolved
// target stays below any allowed root. System targets and non-regular files are rejected.
func (g *ReadGuard) ResolveUnder(base, file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", fmt.Errorf("file is required")
	}
	if !g.ContainsUnder(base) {
		return "", fmt.Errorf("path [%v] is not an allowed directory", base)
	}
	target := file
	if !filepath.IsAbs(target) {
		target = filepath.Join(canonicalReadPath(base), target)
	}
	c := canonicalReadPath(target)
	if IsSystemReadPath(c) {
		return "", fmt.Errorf("path [%v] is a system path", file)
	}
	contained := false
	// Match using the same '/' separators as the compiled roots.
	//
	// Windows accepts both forward slashes and backslashes. Root parsing unifies
	// them to simplify our manual path component splitting, so the target needs
	// the same separator for matching. Keep the native path for filesystem operations.
	matchPath := filepath.ToSlash(c)
	for _, r := range g.roots {
		if r.contains(matchPath, false) {
			contained = true
			break
		}
	}
	if !contained {
		return "", fmt.Errorf("file [%v] is outside of the allowed directories", file)
	}
	fi, err := os.Stat(c)
	if err != nil {
		return "", fmt.Errorf("cannot access file [%v]: %v", file, err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("[%v] is not a regular file", file)
	}
	return c, nil
}
