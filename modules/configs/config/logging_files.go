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
// Both are strictly read-only and confined to the configured log directory.

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
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

func init() {
	api.HandleAPIMethod(api.GET, "/logging/files", listLogFilesAction)
	api.HandleAPIMethod(api.GET, "/logging/tail", tailLogFileAction)
}

const (
	logTailMaxBytes   = 4 * 1024 * 1024 // scan at most the last 4MB of a file
	logTailMaxLines   = 2000
	logReadDirMaxDepth = 6
)

type logFileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`    // relative to the log dir; pass back to /logging/tail
	Size    int64  `json:"size"`
	Updated int64  `json:"updated"` // unix seconds
	Current bool   `json:"current"` // most recently modified file
}

func logDirAbs() (string, error) {
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

// resolveLogFile validates that file (relative to the log dir, or absolute)
// resolves inside the log dir, and returns its absolute path.
func resolveLogFile(file string) (string, error) {
	logDir, err := logDirAbs()
	if err != nil {
		return "", err
	}
	if file == "" {
		return "", fmt.Errorf("file is required")
	}
	target := file
	if !filepath.IsAbs(target) {
		target = filepath.Join(logDir, file)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", err
	}
	// resolve symlinks on both sides to prevent escaping via links
	if resolvedLogDir, err := filepath.EvalSymlinks(logDir); err == nil {
		logDir = resolvedLogDir
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	if resolved != logDir && !strings.HasPrefix(resolved, logDir+string(filepath.Separator)) {
		return "", fmt.Errorf("file [%v] is outside of the log directory", file)
	}
	return resolved, nil
}

func listLogFilesAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	logDir, err := logDirAbs()
	if err != nil {
		api.DefaultAPI.WriteError(w, err.Error(), 500)
		return
	}

	files := []logFileInfo{}
	depth := 0
	_ = filepath.Walk(logDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			depth++
			if depth > logReadDirMaxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		depth = 0
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

	api.DefaultAPI.WriteJSON(w, util.MapStr{
		"log_dir": logDir,
		"files":   files,
	}, 200)
}

func tailLogFileAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	fileParam := req.URL.Query().Get("file")
	absPath, err := resolveLogFile(fileParam)
	if err != nil {
		api.DefaultAPI.WriteError(w, err.Error(), 400)
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
		api.DefaultAPI.WriteError(w, fmt.Sprintf("stat file failed: %v", err), 400)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		api.DefaultAPI.WriteError(w, fmt.Sprintf("open file failed: %v", err), 400)
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
		api.DefaultAPI.WriteError(w, fmt.Sprintf("read file failed: %v", err), 500)
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

	api.DefaultAPI.WriteJSON(w, util.MapStr{
		"file":       fileParam,
		"size":       fi.Size(),
		"updated":    fi.ModTime().Unix(),
		"keyword":    keyword,
		"truncated":  fi.Size() > logTailMaxBytes,
		"lines":      match,
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
