package fs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FilesFromDir returns all non-binary files under dir.
// It uses `git ls-files` when inside a git repo so that .gitignore is
// respected automatically; otherwise it falls back to a plain directory walk
// that skips hidden entries.
func FilesFromDir(dir string) ([]string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving dir %q: %w", dir, err)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}
	if files, err := gitLsFiles(abs); err == nil {
		return files, nil
	}
	return walkDir(abs)
}

// gitLsFiles lists tracked files plus untracked non-ignored files via git.
func gitLsFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "-C", dir, "ls-files",
		"--cached", "--others", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		files = append(files, filepath.Join(dir, filepath.FromSlash(line)))
	}
	return files, nil
}

// walkDir walks dir recursively, skipping hidden files/directories and
// any entries that cannot be accessed due to permission errors.
func walkDir(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil // skip permission-denied entries silently
			}
			return err
		}
		name := info.Name()
		if info.IsDir() {
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// Format reads each path and returns a prompt-ready string where every file
// is wrapped in <file path>…</file path> tags. Paths are displayed relative
// to baseDir. Binary files and permission-denied files are silently skipped.
func Format(paths []string, baseDir string) (string, error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		abs = baseDir
	}
	var sb strings.Builder
	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			if os.IsPermission(err) {
				continue // skip permission-denied files silently
			}
			return "", fmt.Errorf("reading %s: %w", p, err)
		}
		if !utf8.Valid(content) {
			continue // skip non-UTF-8/ASCII files (binary, encoded data, etc.)
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil {
			rel = p
		}
		fmt.Fprintf(&sb, "<file %s>\n%s\n</file %s>\n", rel, content, rel)
	}
	return sb.String(), nil
}