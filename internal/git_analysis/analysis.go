// Package git_analysis provides fast repository analysis using a hybrid
// go-git (metadata) + exec git (log/churn) approach.
package git_analysis

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// CommitInfo holds metadata for the most recent commit.
type CommitInfo struct {
	Hash    string
	Author  string
	Message string
	When    time.Time
}

// RepoStats holds high-level repository statistics.
type RepoStats struct {
	TotalCommits      int
	TotalContributors int
	TotalBranches     int
	TotalTags         int
	RepoSizeBytes     int64
	LatestCommit      *CommitInfo
	RepoPath          string
	RepoName          string
}

// FileChurn holds churn data for a single file.
type FileChurn struct {
	Path          string
	CommitCount   int
	UniqueAuthors int
	LastModified  time.Time
}

// DailyActivity holds commit count for a day.
type DailyActivity struct {
	Date  time.Time
	Count int
}

// ContributorActivity holds commit count per contributor.
type ContributorActivity struct {
	Name  string
	Email string
	Count int
}

// AnalysisResult is the full result of a repository analysis.
type AnalysisResult struct {
	Stats               RepoStats
	FileChurns          []FileChurn
	DailyActivity       []DailyActivity
	ContributorActivity []ContributorActivity
	Error               error
}

// Analyze performs a full analysis of the git repository at repoPath.
func Analyze(repoPath string) AnalysisResult {
	// Resolve the actual .git root using go-git so we handle subdirectory runs.
	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return AnalysisResult{Error: fmt.Errorf("not a git repository: %w", err)}
	}
	wt, err := repo.Worktree()
	if err != nil {
		return AnalysisResult{Error: err}
	}
	rootPath := wt.Filesystem.Root()

	var result AnalysisResult
	result.Stats.RepoPath = rootPath
	result.Stats.RepoName = filepath.Base(rootPath)

	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(4)

	// 1. Overview stats via go-git (fast - just references)
	go func() {
		defer wg.Done()
		stats := computeOverviewStats(repo, rootPath)
		mu.Lock()
		result.Stats.TotalCommits = stats.TotalCommits
		result.Stats.TotalContributors = stats.TotalContributors
		result.Stats.TotalBranches = stats.TotalBranches
		result.Stats.TotalTags = stats.TotalTags
		result.Stats.LatestCommit = stats.LatestCommit
		mu.Unlock()
	}()

	// 2. File churn via git log --name-only (fast - no diffs)
	go func() {
		defer wg.Done()
		churns := computeFileChurn(rootPath)
		mu.Lock()
		result.FileChurns = churns
		mu.Unlock()
	}()

	// 3. Daily activity (last 30 days)
	go func() {
		defer wg.Done()
		daily := computeDailyActivity(rootPath)
		mu.Lock()
		result.DailyActivity = daily
		mu.Unlock()
	}()

	// 4. Contributor activity
	go func() {
		defer wg.Done()
		contributors := computeContributors(rootPath)
		mu.Lock()
		result.ContributorActivity = contributors
		mu.Unlock()
	}()

	wg.Wait()

	result.Stats.RepoSizeBytes = repoSize(repo)

	return result
}

// computeOverviewStats gathers commit count, contributors, branches, tags, and latest commit.
func computeOverviewStats(repo *git.Repository, rootPath string) RepoStats {
	var stats RepoStats

	// Commit count + contributors via git shortlog (very fast)
	out, err := runGit(rootPath, "rev-list", "--count", "HEAD")
	if err == nil {
		n, _ := strconv.Atoi(strings.TrimSpace(out))
		stats.TotalCommits = n
	}

	// Unique contributors
	out, err = runGit(rootPath, "shortlog", "-sn", "--all", "--no-merges")
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) != "" {
				stats.TotalContributors++
			}
		}
	}

	// Branches
	branchIter, err := repo.Branches()
	if err == nil {
		_ = branchIter.ForEach(func(_ *plumbing.Reference) error {
			stats.TotalBranches++
			return nil
		})
	}

	// Tags
	tagIter, err := repo.Tags()
	if err == nil {
		_ = tagIter.ForEach(func(_ *plumbing.Reference) error {
			stats.TotalTags++
			return nil
		})
	}

	// Latest commit
	out, err = runGit(rootPath, "log", "-1", "--format=%h|%an|%s|%ai")
	if err == nil {
		line := strings.TrimSpace(out)
		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			t, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[3])
			stats.LatestCommit = &CommitInfo{
				Hash:    parts[0],
				Author:  parts[1],
				Message: parts[2],
				When:    t,
			}
		}
	}

	return stats
}

// computeFileChurn uses git log --name-only to count per-file commits quickly.
func computeFileChurn(rootPath string) []FileChurn {
	// Format: hash|author_email|author_date then file names on separate lines, blank line between commits
	out, err := runGit(rootPath, "log", "--name-only", "--format=%H|%ae|%ai", "--diff-filter=ACDMRT")
	if err != nil {
		return nil
	}

	type fileData struct {
		commitCount int
		authors     map[string]struct{}
		lastMod     time.Time
	}
	fileMap := make(map[string]*fileData)

	var currentEmail string
	var currentTime time.Time

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Commit header line
		if strings.ContainsRune(line, '|') {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) == 3 {
				currentEmail = parts[1]
				currentTime, _ = time.Parse("2006-01-02 15:04:05 -0700", parts[2])
			}
			continue
		}
		// File name line
		fd, ok := fileMap[line]
		if !ok {
			fd = &fileData{authors: make(map[string]struct{})}
			fileMap[line] = fd
		}
		fd.commitCount++
		fd.authors[currentEmail] = struct{}{}
		if currentTime.After(fd.lastMod) {
			fd.lastMod = currentTime
		}
	}

	churns := make([]FileChurn, 0, len(fileMap))
	for path, fd := range fileMap {
		churns = append(churns, FileChurn{
			Path:          path,
			CommitCount:   fd.commitCount,
			UniqueAuthors: len(fd.authors),
			LastModified:  fd.lastMod,
		})
	}

	sort.Slice(churns, func(i, j int) bool {
		return churns[i].CommitCount > churns[j].CommitCount
	})
	return churns
}

// computeDailyActivity returns commit counts per day for the last 365 days.
func computeDailyActivity(rootPath string) []DailyActivity {
	now := time.Now()
	since := now.AddDate(0, 0, -365).Format("2006-01-02")

	out, err := runGit(rootPath, "log", "--format=%ai", "--since="+since)
	if err != nil {
		return make([]DailyActivity, 365)
	}

	dayMap := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) >= 10 {
			dayMap[line[:10]]++
		}
	}

	daily := make([]DailyActivity, 365)
	for i := 0; i < 365; i++ {
		d := now.AddDate(0, 0, -(364 - i))
		key := d.Format("2006-01-02")
		daily[i] = DailyActivity{Date: d, Count: dayMap[key]}
	}
	return daily
}

// computeContributors returns per-author commit counts across all time.
func computeContributors(rootPath string) []ContributorActivity {
	out, err := runGit(rootPath, "log", "--format=%an|%ae")
	if err != nil {
		return nil
	}

	type entry struct {
		name  string
		count int
	}
	emailMap := make(map[string]*entry)

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "|", 2)
		if len(parts) != 2 {
			continue
		}
		name, email := parts[0], parts[1]
		if e, ok := emailMap[email]; ok {
			e.count++
		} else {
			emailMap[email] = &entry{name: name, count: 1}
		}
	}

	result := make([]ContributorActivity, 0, len(emailMap))
	for email, e := range emailMap {
		result = append(result, ContributorActivity{Name: e.name, Email: email, Count: e.count})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

// repoSize estimates the repo size by summing object pack files.
func repoSize(repo *git.Repository) int64 {
	storer := repo.Storer
	if sizer, ok := storer.(interface{ ObjectStorageSize() (uint64, error) }); ok {
		n, err := sizer.ObjectStorageSize()
		if err == nil {
			return int64(n)
		}
	}
	return 0
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
