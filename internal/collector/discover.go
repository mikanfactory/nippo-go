package collector

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DiscoverSessionFiles walks claudeDir/projects/**/*.jsonl and returns all
// matching files with their modification times. Missing or inaccessible
// entries are skipped silently; a missing projects/ directory returns a
// friendly error because it usually means Claude Code has not been run yet.
func DiscoverSessionFiles(claudeDir string) ([]SessionFile, error) {
	projectsDir := filepath.Join(claudeDir, "projects")
	info, err := os.Stat(projectsDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("Claude Code のセッションデータが見つかりません: %s\n\n"+
			"Claude Code (CLI または IDE 拡張) を使用するとここに JSONL が保存されます。\n"+
			"別のディレクトリを使う場合は --claude-dir オプションを指定してください。", projectsDir)
	}

	var files []SessionFile
	walkErr := filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			// Don't abort on a single unreadable dir — just skip it.
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		fi, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		files = append(files, SessionFile{Path: path, Mtime: fi.ModTime()})
		return nil
	})
	if walkErr != nil {
		return files, fmt.Errorf("failed to walk %s: %w", projectsDir, walkErr)
	}
	return files, nil
}

