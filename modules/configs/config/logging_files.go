/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package config

// Instance-local log viewing API, served by every framework app on its web
// port. Lets a managing console (e.g. LogPilot) tail instance logs through
// its reverse channel for pipeline troubleshooting:
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

func init() {
	// The web-port copy is the secured one (login + RBAC); the API-domain
	// registration stays for existing API-port consumers.
	api.HandleAPIMethod(api.GET, "/logging/files", listLogFilesAction)
	api.HandleUIMethod(api.GET, "/logging/files", listLogFilesAction, api.RequireLogin(), api.RequirePermission(security.PermissionSystemLogRead))
	api.HandleAPIMethod(api.GET, "/logging/tail", tailLogFileAction)
	api.HandleUIMethod(api.GET, "/logging/tail", tailLogFileAction, api.RequireLogin(), api.RequirePermission(security.PermissionSystemLogRead))
}

const (
	logTailMaxBytes    = 4 * 1024 * 1024 // scan at most the last 4MB of a file
	logTailMaxLines    = 2000
	logReadDirMaxDepth = 6
)

type logFileInfo struct {
	Name    string `json:"name"` // file name without directories
	Path    string `json:"path"` // relative to the log dir; pass back to /logging/tail
	Size    int64  `json:"size"`
	Updated int64  `json:"updated"` // unix seconds
	// MostRecentlyModified marks the newest file of the listing; the wire
	// name stays "current" for the console contract.
	MostRecentlyModified bool `json:"current"`
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

// isTailableLogName gates which files both endpoints serve; /logging/files
// only lists these, and /logging/tail rejects anything else so binary side
// files (rotated .gz archives, index snapshots) are never served as text.
func isTailableLogName(name string) bool {
	return strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".json")
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
	resolved, err := guard.ResolveUnder(logDir, file)
	if err != nil {
		return "", err
	}
	if !isTailableLogName(filepath.Base(resolved)) {
		return "", fmt.Errorf("file [%v] is not a log file", file)
	}
	return resolved, nil
}

func listLogFilesAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	logDir, err := currentLogDir()
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// the walk below reports paths as given; make sure logDir itself is
	// canonical so relative paths handed back to /logging/tail match
	logDir, err = filepath.Abs(logDir)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if util.IsSystemReadPath(logDir) {
		api.WriteError(w, fmt.Sprintf("log dir [%v] is a system path", logDir), http.StatusInternalServerError)
		return
	}

	files := []logFileInfo{}
	if err := filepath.Walk(logDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Warnf("skipping [%s] while listing log files: %v", p, err)
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
		if !isTailableLogName(fi.Name()) {
			return nil
		}
		rel, err := filepath.Rel(logDir, p)
		if err != nil {
			log.Warnf("skipping [%s] while listing log files: %v", p, err)
			return nil
		}
		files = append(files, logFileInfo{
			Name:    fi.Name(),
			Path:    filepath.ToSlash(rel),
			Size:    fi.Size(),
			Updated: fi.ModTime().Unix(),
		})
		return nil
	}); err != nil {
		log.Warnf("failed to walk log dir [%s]: %v", logDir, err)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Updated != files[j].Updated {
			return files[i].Updated > files[j].Updated
		}
		return files[i].Name < files[j].Name
	})
	if len(files) > 0 {
		files[0].MostRecentlyModified = true
	}

	api.WriteJSON(w, util.MapStr{
		"log_dir": logDir,
		"files":   files,
	}, http.StatusOK)
}

func tailLogFileAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	fileParam := req.URL.Query().Get("file")
	absPath, err := resolveLogFile(fileParam)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	lines, _ := strconv.Atoi(req.URL.Query().Get("lines"))
	if lines <= 0 {
		lines = 200
	}
	if lines > logTailMaxLines {
		lines = logTailMaxLines
	}
	keyword := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("keyword")))

	fi, err := os.Stat(absPath)
	if err != nil {
		api.WriteError(w, fmt.Sprintf("stat file failed: %v", err), http.StatusBadRequest)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		api.WriteError(w, fmt.Sprintf("open file failed: %v", err), http.StatusBadRequest)
		return
	}
	defer f.Close()

	buf, fullWindow, err := readTailWindow(f, fi.Size())
	if err != nil {
		api.WriteError(w, fmt.Sprintf("read file failed: %v", err), http.StatusInternalServerError)
		return
	}

	// drop the partial first line when we truncated mid-file
	if fullWindow && len(buf) > 0 {
		if idx := bytes.IndexByte(buf, '\n'); idx >= 0 {
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
	}, http.StatusOK)
}

// readTailWindow reads at most logTailMaxBytes from the end of the file.
// A short read — the file shrank between stat and read, as happens with
// rotation or truncation — returns the bytes actually read instead of an
// error or a zero-filled tail; fullWindow reports whether the entire
// window was read (i.e. the window starts mid-file).
func readTailWindow(ra io.ReaderAt, size int64) ([]byte, bool, error) {
	window := int64(logTailMaxBytes)
	if size < window {
		window = size
	}
	if window <= 0 {
		return nil, false, nil
	}
	buf := make([]byte, window)
	n, err := ra.ReadAt(buf, size-window)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, false, err
	}
	return buf[:n], n == len(buf) && window == logTailMaxBytes, nil
}
