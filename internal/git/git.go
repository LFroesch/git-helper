package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Types

type Change struct {
	File   string
	Status string
	Type   string
	Scope  string
}

type Status struct {
	Branch        string
	Clean         bool
	StagedFiles   int
	UnstagedFiles int
	Ahead         int
	Behind        int
}

type Branch struct {
	Name      string
	IsCurrent bool
	IsRemote  bool
	Upstream  string
	Ahead     int
	Behind    int
}

type Commit struct {
	Hash    string
	Message string
	Author  string
	Date    string
}

type ConflictFile struct {
	Path       string
	IsResolved bool
}

type BranchComparison struct {
	SourceBranch   string
	TargetBranch   string
	AheadCommits   []Commit
	BehindCommits  []Commit
	DifferingFiles []string
}

type RebaseCommit struct {
	Hash    string
	Message string
	Action  string
}

type Stash struct {
	Index   int
	Message string
	Date    string
	Branch  string
}

type Tag struct {
	Name        string
	Message     string
	Commit      string
	Date        string
	IsAnnotated bool
}

// Command execution

func Execute(repoPath string, args ...string) ([]byte, error) {
	maxRetries := 3
	retryDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		lockFile := filepath.Join(repoPath, ".git", "index.lock")
		if _, err := os.Stat(lockFile); err == nil {
			time.Sleep(retryDelay)
			continue
		}

		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

		output, err := cmd.CombinedOutput()

		if err != nil && strings.Contains(string(output), "index.lock") {
			time.Sleep(retryDelay)
			retryDelay *= 2
			continue
		}

		return output, err
	}

	return nil, fmt.Errorf("git command failed after %d retries: index.lock conflict", maxRetries)
}

func IsRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// Status functions

func GetBranchName(repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	return "unknown"
}

func GetAheadBehindCount(repoPath string) (ahead, behind int) {
	// Use git status -sb which reliably shows ahead/behind even without explicit upstream
	cmd := exec.Command("git", "status", "-sb")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	// Parse first line: ## branch...origin/branch [ahead N, behind M]
	firstLine := strings.Split(string(output), "\n")[0]

	// Look for [ahead N] or [behind N] or [ahead N, behind M]
	if idx := strings.Index(firstLine, "["); idx != -1 {
		if endIdx := strings.Index(firstLine[idx:], "]"); endIdx != -1 {
			trackInfo := firstLine[idx+1 : idx+endIdx]
			// Parse "ahead N" and/or "behind M"
			if strings.Contains(trackInfo, "ahead") {
				fmt.Sscanf(trackInfo, "ahead %d", &ahead)
			}
			if strings.Contains(trackInfo, "behind") {
				if strings.Contains(trackInfo, "ahead") {
					// Format: "ahead N, behind M"
					parts := strings.Split(trackInfo, ", ")
					for _, part := range parts {
						if strings.HasPrefix(part, "behind") {
							fmt.Sscanf(part, "behind %d", &behind)
						}
					}
				} else {
					fmt.Sscanf(trackInfo, "behind %d", &behind)
				}
			}
		}
	}

	return ahead, behind
}

func GetStatus(repoPath string) Status {
	status := Status{Branch: GetBranchName(repoPath)}
	status.Ahead, status.Behind = GetAheadBehindCount(repoPath)

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return status
	}

	statusText := strings.TrimSpace(string(output))
	status.Clean = statusText == ""

	if !status.Clean {
		lines := strings.Split(statusText, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if len(line) >= 2 {
				stagedStatus := line[0]
				unstagedStatus := line[1]

				if stagedStatus != ' ' && stagedStatus != '?' {
					status.StagedFiles++
				}
				if unstagedStatus != ' ' {
					status.UnstagedFiles++
				}
			}
		}
	}

	return status
}

func GetChanges(repoPath string) []Change {
	var changes []Change

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return changes
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || len(line) < 3 {
			continue
		}

		status := line[:2]
		file := strings.TrimSpace(line[3:])

		changes = append(changes, Change{
			File:   file,
			Status: status,
		})
	}

	return changes
}

// Branch functions

func GetBranches(repoPath string) []Branch {
	var branches []Branch

	// Local branches
	cmd := exec.Command("git", "branch", "-vv")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return branches
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		branch := Branch{}
		branch.IsCurrent = strings.HasPrefix(line, "*")

		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "  ")

		parts := strings.Fields(line)
		if len(parts) >= 1 {
			branch.Name = parts[0]
		}

		// Parse upstream tracking info [origin/main: ahead 1, behind 2]
		if idx := strings.Index(line, "["); idx != -1 {
			if endIdx := strings.Index(line[idx:], "]"); endIdx != -1 {
				trackingInfo := line[idx+1 : idx+endIdx]
				if colonIdx := strings.Index(trackingInfo, ":"); colonIdx != -1 {
					branch.Upstream = strings.TrimSpace(trackingInfo[:colonIdx])
					status := trackingInfo[colonIdx+1:]
					if strings.Contains(status, "ahead") {
						fmt.Sscanf(status, " ahead %d", &branch.Ahead)
					}
					if strings.Contains(status, "behind") {
						if strings.Contains(status, "ahead") {
							fmt.Sscanf(status, " ahead %d, behind %d", &branch.Ahead, &branch.Behind)
						} else {
							fmt.Sscanf(status, " behind %d", &branch.Behind)
						}
					}
				} else {
					branch.Upstream = strings.TrimSpace(trackingInfo)
				}
			}
		}

		branches = append(branches, branch)
	}

	return branches
}

func GetRemoteBranches(repoPath string) []Branch {
	var branches []Branch

	cmd := exec.Command("git", "branch", "-r")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return branches
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "->") {
			continue
		}

		branches = append(branches, Branch{
			Name:     line,
			IsRemote: true,
		})
	}

	return branches
}

func HasRemoteBranch(repoPath, branchName string) bool {
	cmd := exec.Command("git", "ls-remote", "--heads", "origin", branchName)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(output))) > 0
}

// Commit functions

func GetCommitLog(repoPath string, count int) []Commit {
	var commits []Commit

	cmd := exec.Command("git", "log", fmt.Sprintf("-%d", count), "--pretty=format:%h|%s|%an|%ar")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return commits
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 4 {
			commits = append(commits, Commit{
				Hash:    parts[0],
				Message: parts[1],
				Author:  parts[2],
				Date:    parts[3],
			})
		}
	}

	return commits
}

func GetReflog(repoPath string, count int) []Commit {
	var commits []Commit

	cmd := exec.Command("git", "reflog", fmt.Sprintf("-%d", count), "--pretty=format:%h|%s|%ar")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return commits
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) >= 3 {
			commits = append(commits, Commit{
				Hash:    parts[0],
				Message: parts[1],
				Date:    parts[2],
			})
		}
	}

	return commits
}

func GetCurrentCommitHash(repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// Staging functions

func IsFileStaged(repoPath, filePath string) bool {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	stagedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, f := range stagedFiles {
		if strings.TrimSpace(f) == filePath {
			return true
		}
	}
	return false
}

func GetStagedFiles(repoPath string) []string {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func GetStagedDiff(repoPath string) string {
	cmd := exec.Command("git", "diff", "--cached")
	cmd.Dir = repoPath
	output, _ := cmd.Output()
	return string(output)
}

// Diff functions

func GetFileDiff(repoPath, filePath string, staged bool) string {
	var cmd *exec.Cmd
	if staged {
		cmd = exec.Command("git", "diff", "--cached", filePath)
	} else {
		cmd = exec.Command("git", "diff", filePath)
	}
	cmd.Dir = repoPath
	output, _ := cmd.Output()
	return string(output)
}

// Conflict functions

func GetConflictFiles(repoPath string) []string {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// Comparison functions

func GetBranchComparison(repoPath, sourceBranch, targetBranch string) BranchComparison {
	comparison := BranchComparison{
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
	}

	// Ahead commits
	cmd := exec.Command("git", "log", "--pretty=format:%h|%s|%an|%ar", targetBranch+"..HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 4)
			if len(parts) >= 4 {
				comparison.AheadCommits = append(comparison.AheadCommits, Commit{
					Hash:    parts[0],
					Message: parts[1],
					Author:  parts[2],
					Date:    parts[3],
				})
			}
		}
	}

	// Behind commits
	cmd = exec.Command("git", "log", "--pretty=format:%h|%s|%an|%ar", "HEAD.."+targetBranch)
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 4)
			if len(parts) >= 4 {
				comparison.BehindCommits = append(comparison.BehindCommits, Commit{
					Hash:    parts[0],
					Message: parts[1],
					Author:  parts[2],
					Date:    parts[3],
				})
			}
		}
	}

	// Differing files
	cmd = exec.Command("git", "diff", "--name-only", targetBranch+"...HEAD")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err == nil {
		text := strings.TrimSpace(string(output))
		if text != "" {
			comparison.DifferingFiles = strings.Split(text, "\n")
		}
	}

	return comparison
}

// Stash functions

func GetStashList(repoPath string) []Stash {
	var stashes []Stash

	cmd := exec.Command("git", "stash", "list", "--format=%gd|%s|%ar")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return stashes
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) >= 3 {
			stashes = append(stashes, Stash{
				Index:   i,
				Message: parts[1],
				Date:    parts[2],
			})
		}
	}

	return stashes
}

func StashPush(repoPath, message string) error {
	var args []string
	if message != "" {
		args = []string{"stash", "push", "-m", message}
	} else {
		args = []string{"stash", "push"}
	}
	_, err := Execute(repoPath, args...)
	return err
}

func StashPop(repoPath string, index int) error {
	_, err := Execute(repoPath, "stash", "pop", fmt.Sprintf("stash@{%d}", index))
	return err
}

func StashApply(repoPath string, index int) error {
	_, err := Execute(repoPath, "stash", "apply", fmt.Sprintf("stash@{%d}", index))
	return err
}

func StashDrop(repoPath string, index int) error {
	_, err := Execute(repoPath, "stash", "drop", fmt.Sprintf("stash@{%d}", index))
	return err
}

func StashShow(repoPath string, index int) string {
	cmd := exec.Command("git", "stash", "show", "-p", fmt.Sprintf("stash@{%d}", index))
	cmd.Dir = repoPath
	output, _ := cmd.Output()
	return string(output)
}

// Tag functions

func GetTags(repoPath string) []Tag {
	var tags []Tag

	// Get all tags with their details
	cmd := exec.Command("git", "tag", "-l", "--format=%(refname:short)|%(objecttype)|%(creatordate:relative)|%(*objectname:short)%(objectname:short)")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return tags
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 4 {
			tag := Tag{
				Name:        parts[0],
				IsAnnotated: parts[1] == "tag",
				Date:        parts[2],
				Commit:      parts[3],
			}

			// Get message for annotated tags
			if tag.IsAnnotated {
				msgCmd := exec.Command("git", "tag", "-l", "--format=%(contents:subject)", tag.Name)
				msgCmd.Dir = repoPath
				msgOutput, _ := msgCmd.Output()
				tag.Message = strings.TrimSpace(string(msgOutput))
			}

			tags = append(tags, tag)
		}
	}

	return tags
}

func CreateTag(repoPath, name, message string, annotated bool) error {
	var args []string
	if annotated && message != "" {
		args = []string{"tag", "-a", name, "-m", message}
	} else {
		args = []string{"tag", name}
	}
	_, err := Execute(repoPath, args...)
	return err
}

func DeleteTag(repoPath, name string) error {
	_, err := Execute(repoPath, "tag", "-d", name)
	return err
}

func PushTag(repoPath, name string) error {
	_, err := Execute(repoPath, "push", "origin", name)
	return err
}

func PushAllTags(repoPath string) error {
	_, err := Execute(repoPath, "push", "--tags")
	return err
}

// Cherry-pick and Revert functions

func CherryPick(repoPath, commitHash string) error {
	_, err := Execute(repoPath, "cherry-pick", commitHash)
	return err
}

func CherryPickAbort(repoPath string) error {
	_, err := Execute(repoPath, "cherry-pick", "--abort")
	return err
}

func CherryPickContinue(repoPath string) error {
	_, err := Execute(repoPath, "cherry-pick", "--continue")
	return err
}

func RevertCommit(repoPath, commitHash string) error {
	_, err := Execute(repoPath, "revert", "--no-edit", commitHash)
	return err
}

func RevertAbort(repoPath string) error {
	_, err := Execute(repoPath, "revert", "--abort")
	return err
}

// Clean functions

func CleanDryRun(repoPath string) ([]string, error) {
	output, err := Execute(repoPath, "clean", "-n", "-d")
	if err != nil {
		return nil, err
	}

	var files []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimPrefix(line, "Would remove ")
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func CleanForce(repoPath string) error {
	_, err := Execute(repoPath, "clean", "-f", "-d")
	return err
}

// Clone and Init functions

func Clone(url, targetPath string) (string, error) {
	cmd := exec.Command("git", "clone", url, targetPath)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func Init(path string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = path
	_, err := cmd.CombinedOutput()
	return err
}

// Log viewer functions

type CommitDetail struct {
	Hash       string
	Message    string
	Body       string
	Author     string
	Email      string
	Date       string
	Files      []string
	Insertions int
	Deletions  int
}

func GetCommitLog2(repoPath string, count int, search string) []Commit {
	var commits []Commit
	args := []string{"log", fmt.Sprintf("-%d", count), "--pretty=format:%h|%s|%an|%ar"}
	if search != "" {
		args = append(args, "--grep="+search)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return commits
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 4 {
			commits = append(commits, Commit{
				Hash:    parts[0],
				Message: parts[1],
				Author:  parts[2],
				Date:    parts[3],
			})
		}
	}
	return commits
}

func GetCommitDetail(repoPath, hash string) CommitDetail {
	detail := CommitDetail{Hash: hash}

	// Get commit info
	cmd := exec.Command("git", "show", hash, "--pretty=format:%H|%s|%b|%an|%ae|%ar", "--stat")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return detail
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		parts := strings.SplitN(lines[0], "|", 6)
		if len(parts) >= 6 {
			detail.Hash = parts[0]
			detail.Message = parts[1]
			detail.Body = parts[2]
			detail.Author = parts[3]
			detail.Email = parts[4]
			detail.Date = parts[5]
		}
	}

	// Parse file stats
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.Contains(line, "changed") {
			// Summary line: "2 files changed, 10 insertions(+), 5 deletions(-)"
			fmt.Sscanf(line, "%*d file%*s changed, %d insertion%*s %d deletion", &detail.Insertions, &detail.Deletions)
		} else if strings.Contains(line, "|") {
			// File line: "filename | 5 ++-"
			parts := strings.Split(line, "|")
			if len(parts) >= 1 {
				detail.Files = append(detail.Files, strings.TrimSpace(parts[0]))
			}
		}
	}

	return detail
}

func GetCommitDiff(repoPath, hash string) string {
	cmd := exec.Command("git", "show", hash, "--pretty=format:", "--patch")
	cmd.Dir = repoPath
	output, _ := cmd.Output()
	return string(output)
}

// Interactive Rebase functions

func ExecuteRebase(repoPath string, commits []RebaseCommit) error {
	if len(commits) == 0 {
		return fmt.Errorf("no commits to rebase")
	}

	// Build rebase todo content (oldest first, so reverse the slice)
	var todoLines []string
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]
		action := commit.Action
		if action == "" {
			action = "pick"
		}
		todoLines = append(todoLines, fmt.Sprintf("%s %s %s", action, commit.Hash, commit.Message))
	}
	todoContent := strings.Join(todoLines, "\n") + "\n"

	// Write todo to temp file
	tmpFile, err := os.CreateTemp("", "gitty-rebase-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(todoContent); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write rebase todo: %w", err)
	}
	tmpFile.Close()

	// Create editor script that copies our todo file
	editorScript := fmt.Sprintf("cp %s \"$1\"", tmpPath)

	// Run git rebase with our custom editor
	count := len(commits)
	cmd := exec.Command("git", "rebase", "-i", fmt.Sprintf("HEAD~%d", count))
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR=sh -c '"+editorScript+"'")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase failed: %s", string(output))
	}

	return nil
}

func AbortRebase(repoPath string) error {
	_, err := Execute(repoPath, "rebase", "--abort")
	return err
}

func ContinueRebase(repoPath string) error {
	_, err := Execute(repoPath, "rebase", "--continue")
	return err
}

func IsRebaseInProgress(repoPath string) bool {
	rebaseMerge := filepath.Join(repoPath, ".git", "rebase-merge")
	rebaseApply := filepath.Join(repoPath, ".git", "rebase-apply")
	_, err1 := os.Stat(rebaseMerge)
	_, err2 := os.Stat(rebaseApply)
	return err1 == nil || err2 == nil
}

// Blame functions

type BlameLine struct {
	Hash    string
	Author  string
	Date    string
	LineNum int
	Content string
}

func GetBlame(repoPath, filePath string) []BlameLine {
	var lines []BlameLine

	cmd := exec.Command("git", "blame", "--porcelain", filePath)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return lines
	}

	rawLines := strings.Split(string(output), "\n")
	var currentHash, currentAuthor, currentDate string
	lineNum := 0

	for i := 0; i < len(rawLines); i++ {
		line := rawLines[i]
		if line == "" {
			continue
		}

		// Hash line: starts with 40-char hash
		if len(line) >= 40 && !strings.HasPrefix(line, "\t") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				currentHash = parts[0][:7] // short hash
			}
		} else if strings.HasPrefix(line, "author ") {
			currentAuthor = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-time ") {
			ts := strings.TrimPrefix(line, "author-time ")
			if timestamp, err := strconv.ParseInt(ts, 10, 64); err == nil {
				t := time.Unix(timestamp, 0)
				currentDate = t.Format("2006-01-02")
			}
		} else if strings.HasPrefix(line, "\t") {
			lineNum++
			lines = append(lines, BlameLine{
				Hash:    currentHash,
				Author:  currentAuthor,
				Date:    currentDate,
				LineNum: lineNum,
				Content: strings.TrimPrefix(line, "\t"),
			})
		}
	}

	return lines
}
