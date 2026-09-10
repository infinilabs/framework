/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package config

// Instance-local log viewing API, served by every framework app next to the
// /config/ endpoints (embedded API on agents/gateways). Lets a managing
// console (e.g. LogPilot) tail instance logs through its reverse channel for
// pipeline troubleshooting:
//
//	GET /logging/files                     list log files (recursive, newest first)
//	GET /logging/tail?file=&lines=&keyword=  tail a file, optional keyword filter
//
// Both are strictly read-only. File access goes through util.ReadGuard with
// the app's own log directory as the only allowed root: paths are
// symlink-resolved and confined to that directory, system paths
// (util.IsSystemReadPath) are denied outright, and only regular files are
// served — the endpoints can never be turned into a general-purpose file
// reader.

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"infini.sh/framework/core/api"
	"infini.sh/framework/core/security"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// logFilesRead gates the log list/tail endpoints; managers mint the
// instance self API token with this key (see core/security instance-ops
// catalog) so reverse-channel loopback calls pass the permission filter.
var logFilesRead = security.GetSimplePermission("generic", "system:log", security.Read)

func init() {
	// The web-port copy is the secured one (login + RBAC); the API-domain
	// registration stays for existing API-port consumers.
	api.HandleAPIMethod(api.GET, "/logging/files", listLogFilesAction)
	api.HandleUIMethod(api.GET, "/logging/files", listLogFilesAction, api.RequireLogin(), api.RequirePermission(logFilesRead))
	api.HandleAPIMethod(api.GET, "/logging/tail", tailLogFileAction)
	api.HandleUIMethod(api.GET, "/logging/tail", tailLogFileAction, api.RequireLogin(), api.RequirePermission(logFilesRead))
}

const (
	logTailMaxBytes    = 4 * 1024 * 1024 // scan at most the last 4MB of a file
	logTailMaxLines    = 2000
	logReadDirMaxDepth = 6
)

type logFileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"` // relative to the log dir; pass back to /logging/tail
	Size    int64  `json:"size"`
	Updated int64  `json:"updated"` // unix seconds
	Current bool   `json:"current"` // most recently modified file
}

// currentLogDir resolves the directory the /logging endpoints serve;
// overridable in tests.
var currentLogDir = func() (string, error) {
	dir := global.Env().GetLogDir()
	if dir == "" {
		return "", fmt.Errorf("log dir is not configured")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// resolveLogFile validates that file (relative to the log dir, or absolute
// inside it) resolves to a regular file strictly inside the log directory,
// and returns its absolute path.
func resolveLogFile(file string) (string, error) {
	logDir, err := currentLogDir()
	if err != nil {
		return "", err
	}
	guard, err := util.NewReadGuard(logDir)
	if err != nil {
		return "", err
	}
	return guard.ResolveUnder(logDir, file)
}

func listLogFilesAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	logDir, err := currentLogDir()
	if err != nil {
		api.WriteError(w, err.Error(), 500)
		return
	}
	// the walk below reports paths as given; make sure logDir itself is
	// canonical so relative paths handed back to /logging/tail match
	logDir, err = filepath.Abs(logDir)
	if err != nil {
		api.WriteError(w, err.Error(), 500)
		return
	}
	if util.IsSystemReadPath(logDir) {
		api.WriteError(w, fmt.Sprintf("log dir [%v] is a system path", logDir), 500)
		return
	}

	files := []logFileInfo{}
	_ = filepath.Walk(logDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			if p == logDir {
				return nil
			}
			// depth is derived from the relative path so sibling
			// directories are not pruned by an earlier deep subtree
			if rel, err := filepath.Rel(logDir, p); err == nil &&
				strings.Count(rel, string(filepath.Separator))+1 > logReadDirMaxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		if !strings.HasSuffix(fi.Name(), ".log") && !strings.HasSuffix(fi.Name(), ".json") {
			return nil
		}
		rel, err := filepath.Rel(logDir, p)
		if err != nil {
			return nil
		}
		files = append(files, logFileInfo{
			Name:    filepath.ToSlash(rel),
			Path:    filepath.ToSlash(rel),
			Size:    fi.Size(),
			Updated: fi.ModTime().Unix(),
		})
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		if files[i].Updated != files[j].Updated {
			return files[i].Updated > files[j].Updated
		}
		return files[i].Name < files[j].Name
	})
	if len(files) > 0 {
		files[0].Current = true
	}

	api.WriteJSON(w, util.MapStr{
		"log_dir": logDir,
		"files":   files,
	}, 200)
}

func tailLogFileAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	fileParam := req.URL.Query().Get("file")
	absPath, err := resolveLogFile(fileParam)
	if err != nil {
		api.WriteError(w, err.Error(), 400)
		return
	}

	lines, _ := strconv.Atoi(req.URL.Query().Get("lines"))
	if lines == 0 {
		lines = 200
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > logTailMaxLines {
		lines = logTailMaxLines
	}
	keyword := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("keyword")))

	fi, err := os.Stat(absPath)
	if err != nil {
		api.WriteError(w, fmt.Sprintf("stat file failed: %v", err), 400)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		api.WriteError(w, fmt.Sprintf("open file failed: %v", err), 400)
		return
	}
	defer f.Close()

	// read a bounded window from the end of the file
	window := int64(logTailMaxBytes)
	if fi.Size() < window {
		window = fi.Size()
	}
	buf := make([]byte, window)
	if _, err = f.ReadAt(buf, fi.Size()-window); err != nil && err.Error() != "EOF" {
		api.WriteError(w, fmt.Sprintf("read file failed: %v", err), 500)
		return
	}

	// drop the partial first line when we truncated mid-file
	if window == logTailMaxBytes && len(buf) > 0 {
		if idx := indexByte(buf, '\n'); idx >= 0 {
			buf = buf[idx+1:]
		}
	}

	rawLines := strings.Split(strings.ReplaceAll(string(buf), "\r\n", "\n"), "\n")
	if n := len(rawLines); n > 0 && rawLines[n-1] == "" {
		rawLines = rawLines[:n-1]
	}

	match := make([]string, 0, lines)
	if keyword == "" {
		if len(rawLines) > lines {
			match = rawLines[len(rawLines)-lines:]
		} else {
			match = rawLines
		}
	} else {
		for i := len(rawLines) - 1; i >= 0 && len(match) < lines; i-- {
			if strings.Contains(strings.ToLower(rawLines[i]), keyword) {
				match = append(match, rawLines[i])
			}
		}
		// reversed above, restore chronological order
		for i, j := 0, len(match)-1; i < j; i, j = i+1, j-1 {
			match[i], match[j] = match[j], match[i]
		}
	}

	api.WriteJSON(w, util.MapStr{
		"file":        fileParam,
		"size":        fi.Size(),
		"updated":     fi.ModTime().Unix(),
		"keyword":     keyword,
		"truncated":   fi.Size() > logTailMaxBytes,
		"lines":       match,
		"server_time": time.Now().Unix(),
	}, 200)
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}
