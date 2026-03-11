package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gg "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/pmezard/go-difflib/difflib"
)

type Ops struct{ Root string }

func NewOps() *Ops {
	return NewOpsWithRoot("")
}

func NewOpsWithRoot(rootDir string) *Ops {
	wd := strings.TrimSpace(rootDir)
	if wd == "" {
		wd, _ = os.Getwd()
	}
	root := wd
	cur := wd
	for {
		if fi, err := os.Stat(filepath.Join(cur, ".git")); err == nil && fi.IsDir() {
			root = cur
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return &Ops{Root: root}
}

func (o *Ops) repo() (*gg.Repository, error) {
	r, err := gg.PlainOpen(o.Root)
	if err == gg.ErrRepositoryNotExists {
		return nil, err
	}
	return r, err
}

func (o *Ops) Init() (string, error) {
	_, err := gg.PlainInit(o.Root, false)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(o.Root), nil
}

type Change struct {
	Path  string
	State string
}

func (o *Ops) Status() ([]Change, error) {
	// 使用系统 git 命令，以获得最准确的状态（尊重用户配置如 core.autocrlf）
	// 添加超时控制：30秒
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = o.Root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var changes []Change
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}
		// Porcelain format: XY PATH
		xy := line[:2]
		path := line[3:]
		// 去除可能的引号
		path = strings.Trim(path, "\"")

		state := ""
		if xy == "??" {
			state = "untracked"
		} else if xy[1] == 'M' {
			state = "modified"
		} else if xy[0] == 'A' || xy[0] == 'M' {
			state = "staged"
		} else if xy[1] == 'D' || xy[0] == 'D' {
			state = "deleted"
		} else {
			state = "modified"
		}

		changes = append(changes, Change{Path: filepath.ToSlash(path), State: state})
	}
	return changes, nil
}

func (o *Ops) Add(paths []string) (int, error) {
	r, err := o.repo()
	if err != nil {
		return 0, err
	}
	wt, err := r.Worktree()
	if err != nil {
		return 0, err
	}
	cnt := 0
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if len(paths) == 1 && strings.TrimSpace(paths[0]) == "." {
		st, e := wt.Status()
		if e != nil {
			return 0, e
		}
		for p, s := range st {
			changed := s.Worktree == gg.Modified || s.Worktree == gg.Untracked || s.Staging == gg.Modified || s.Staging == gg.Untracked || s.Staging == gg.Added
			if !changed {
				continue
			}
			_, e2 := wt.Add(p)
			if e2 == nil {
				cnt++
			}
		}
		return cnt, nil
	}
	for _, p := range paths {
		ap := p
		if !filepath.IsAbs(ap) {
			ap = filepath.Join(o.Root, filepath.FromSlash(p))
		}
		rel, e := filepath.Rel(o.Root, ap)
		if e != nil {
			rel = p
		}
		_, e2 := wt.Add(rel)
		if e2 == nil {
			cnt++
		}
	}
	return cnt, nil
}

type CommitOut struct {
	Hash  string
	Files []string
}

func (o *Ops) Commit(message, name, email string) (*CommitOut, error) {
	r, err := o.repo()
	if err != nil {
		return nil, err
	}
	wt, err := r.Worktree()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = os.Getenv("GIT_AUTHOR_NAME")
	}
	if strings.TrimSpace(email) == "" {
		email = os.Getenv("GIT_AUTHOR_EMAIL")
	}
	if strings.TrimSpace(name) == "" {
		name = "AI"
	}
	if strings.TrimSpace(email) == "" {
		email = "ai@example.com"
	}
	h, err := wt.Commit(message, &gg.CommitOptions{Author: &object.Signature{Name: name, Email: email, When: time.Now()}})
	if err != nil {
		return nil, err
	}
	co, err := r.CommitObject(h)
	if err != nil {
		return nil, err
	}
	fs := []string{}
	tree, e := co.Tree()
	if e == nil {
		_ = tree.Files().ForEach(func(f *object.File) error { fs = append(fs, filepath.ToSlash(f.Name)); return nil })
	}
	return &CommitOut{Hash: h.String(), Files: fs}, nil
}

func (o *Ops) BranchList() ([]string, string, error) {
	r, err := o.repo()
	if err != nil {
		return nil, "", err
	}
	it, err := r.Branches()
	if err != nil {
		return nil, "", err
	}
	var bs []string
	_ = it.ForEach(func(ref *plumbing.Reference) error {
		bs = append(bs, strings.TrimPrefix(ref.Name().String(), "refs/heads/"))
		return nil
	})
	head, _ := r.Head()
	cur := ""
	if head != nil {
		cur = strings.TrimPrefix(head.Name().String(), "refs/heads/")
	}
	return bs, cur, nil
}

func (o *Ops) Checkout(name string, create bool) (string, error) {
	r, err := o.repo()
	if err != nil {
		return "", err
	}
	wt, err := r.Worktree()
	if err != nil {
		return "", err
	}
	br := plumbing.NewBranchReferenceName(name)
	opt := &gg.CheckoutOptions{Branch: br, Create: create}
	if err := wt.Checkout(opt); err != nil {
		return "", err
	}
	return name, nil
}

func (o *Ops) Pull(remote, branch, user, pass string) (string, error) {
	r, err := o.repo()
	if err != nil {
		return "", err
	}
	wt, err := r.Worktree()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}
	var ref plumbing.ReferenceName
	if strings.TrimSpace(branch) != "" {
		ref = plumbing.NewBranchReferenceName(branch)
	}
	var auth *http.BasicAuth
	if strings.TrimSpace(user) != "" {
		auth = &http.BasicAuth{Username: user, Password: pass}
	}
	e := wt.Pull(&gg.PullOptions{RemoteName: remote, ReferenceName: ref, Auth: auth, Force: false})
	if e == gg.NoErrAlreadyUpToDate {
		return "up-to-date", nil
	}
	if e != nil {
		return "", e
	}
	return "pulled", nil
}

func (o *Ops) Push(remote, branch, user, pass string) (string, error) {
	r, err := o.repo()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}
	var auth *http.BasicAuth
	if strings.TrimSpace(user) != "" {
		auth = &http.BasicAuth{Username: user, Password: pass}
	}
	e := r.Push(&gg.PushOptions{RemoteName: remote, Auth: auth})
	if e == gg.NoErrAlreadyUpToDate {
		return "up-to-date", nil
	}
	if e != nil {
		return "", e
	}
	return "pushed", nil
}

func (o *Ops) Diff(path string) (string, error) {
	r, err := o.repo()
	if err != nil {
		return "", err
	}
	head, err := r.Head()
	if err != nil {
		return "", err
	}
	co, err := r.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}
	tree, err := co.Tree()
	if err != nil {
		return "", err
	}
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(o.Root, filepath.FromSlash(path))
	}
	rel, _ := filepath.Rel(o.Root, p)
	rel = filepath.ToSlash(rel)
	var old string
	if f, e := tree.File(rel); e == nil {
		rc, e2 := f.Blob.Reader()
		if e2 == nil {
			defer func() { _ = rc.Close() }()
			b := new(strings.Builder)
			buf := make([]byte, 4096)
			for {
				n, er := rc.Read(buf)
				if n > 0 {
					b.Write(buf[:n])
				}
				if er != nil {
					break
				}
			}
			old = b.String()
		}
	}
	bs, e3 := os.ReadFile(p)
	if e3 != nil {
		return "", e3
	}
	newc := string(bs)
	a := []string{}
	b := []string{}
	for _, s := range strings.Split(old, "\n") {
		a = append(a, s+"\n")
	}
	for _, s := range strings.Split(newc, "\n") {
		b = append(b, s+"\n")
	}
	ud := difflib.UnifiedDiff{A: a, B: b, FromFile: "a/" + rel, ToFile: "b/" + rel, Context: 3}
	txt, e4 := difflib.GetUnifiedDiffString(ud)
	if e4 != nil {
		return "", e4
	}
	return txt, nil
}

type LogEntry struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
}

type LogOut struct {
	Branch  string     `json:"branch"`
	Entries []LogEntry `json:"entries"`
	Text    string     `json:"text"`
}

func (o *Ops) Log(limit int, oneline, graph, all bool, path string) (*LogOut, error) {
	if limit <= 0 {
		limit = 20
	}

	args := []string{"log", fmt.Sprintf("--max-count=%d", limit)}
	if oneline {
		args = append(args, "--oneline")
	}
	if graph {
		args = append(args, "--graph")
	}
	if all {
		args = append(args, "--all")
	}
	if strings.TrimSpace(path) != "" {
		pp := strings.TrimSpace(path)
		if filepath.IsAbs(pp) {
			if rel, e := filepath.Rel(o.Root, pp); e == nil {
				pp = rel
			}
		}
		pp = filepath.ToSlash(pp)
		args = append(args, "--", pp)
	}

	txt, err := o.runGitWithTimeout(30*time.Second, args...)
	if err != nil {
		return nil, err
	}

	branch, _ := o.branchName()
	entries := parseGitLogEntries(txt)
	return &LogOut{Branch: branch, Entries: entries, Text: txt}, nil
}

type ShowOut struct {
	Branch   string `json:"branch"`
	Revision string `json:"revision"`
	Text     string `json:"text"`
}

func (o *Ops) Show(revision, path string) (*ShowOut, error) {
	rev := strings.TrimSpace(revision)
	if rev == "" {
		rev = "HEAD"
	}
	args := []string{"show", rev}
	pp := strings.TrimSpace(path)
	if pp != "" {
		if filepath.IsAbs(pp) {
			if rel, e := filepath.Rel(o.Root, pp); e == nil {
				pp = rel
			}
		}
		pp = filepath.ToSlash(pp)
		args = append(args, "--", pp)
	}

	txt, err := o.runGitWithTimeout(30*time.Second, args...)
	if err != nil {
		return nil, err
	}
	branch, _ := o.branchName()
	return &ShowOut{Branch: branch, Revision: rev, Text: txt}, nil
}

type StashEntry struct {
	Index   int    `json:"index"`
	Ref     string `json:"ref"`
	Branch  string `json:"branch"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}

type StashOut struct {
	Branch  string       `json:"branch"`
	Stashes []StashEntry `json:"stashes,omitempty"`
	Text    string       `json:"text"`
}

func (o *Ops) Stash(action, message string, index int, includeUntracked bool) (*StashOut, error) {
	act := strings.ToLower(strings.TrimSpace(action))
	if act == "" {
		return nil, fmt.Errorf("action required")
	}

	var args []string
	switch act {
	case "list":
		args = []string{"stash", "list"}
	case "save":
		args = []string{"stash", "push"}
		if includeUntracked {
			args = append(args, "-u")
		}
		if strings.TrimSpace(message) != "" {
			args = append(args, "-m", message)
		}
	case "pop", "apply", "drop":
		ref := fmt.Sprintf("stash@{%d}", index)
		args = []string{"stash", act, ref}
	default:
		return nil, fmt.Errorf("unsupported action: %s", act)
	}

	txt, err := o.runGitWithTimeout(2*time.Minute, args...)
	if err != nil {
		return nil, err
	}

	branch, _ := o.branchName()
	out := &StashOut{Branch: branch, Text: txt}
	if act == "list" {
		out.Stashes = parseGitStashList(txt)
	}
	return out, nil
}

type ResetOut struct {
	Branch   string `json:"branch"`
	Mode     string `json:"mode"`
	Target   string `json:"target"`
	HeadHash string `json:"head_hash"`
	Text     string `json:"text"`
}

func (o *Ops) Reset(mode, target string) (*ResetOut, error) {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = "mixed"
	}
	switch m {
	case "soft", "mixed", "hard":
	default:
		return nil, fmt.Errorf("unsupported mode: %s", m)
	}
	t := strings.TrimSpace(target)
	if t == "" {
		return nil, fmt.Errorf("target required")
	}

	txt, err := o.runGitWithTimeout(2*time.Minute, "reset", "--"+m, t)
	if err != nil {
		return nil, err
	}

	branch, _ := o.branchName()
	headHash, _ := o.headHash()
	return &ResetOut{Branch: branch, Mode: m, Target: t, HeadHash: headHash, Text: txt}, nil
}

type RevertOut struct {
	Branch   string `json:"branch"`
	Commit   string `json:"commit"`
	HeadHash string `json:"head_hash"`
	Text     string `json:"text"`
}

func (o *Ops) Revert(commit string, noEdit bool, mainline int) (*RevertOut, error) {
	c := strings.TrimSpace(commit)
	if c == "" {
		return nil, fmt.Errorf("commit required")
	}

	args := []string{"revert"}
	if noEdit {
		args = append(args, "--no-edit")
	}
	if mainline > 0 {
		args = append(args, "-m", fmt.Sprintf("%d", mainline))
	}
	args = append(args, c)

	txt, err := o.runGitWithTimeout(2*time.Minute, args...)
	if err != nil {
		return nil, err
	}

	branch, _ := o.branchName()
	headHash, _ := o.headHash()
	return &RevertOut{Branch: branch, Commit: c, HeadHash: headHash, Text: txt}, nil
}

type MergeOut struct {
	Branch       string `json:"branch"`
	MergedBranch string `json:"merged_branch"`
	HeadHash     string `json:"head_hash"`
	Text         string `json:"text"`
}

func (o *Ops) Merge(branch string, noEdit bool, noFF bool) (*MergeOut, error) {
	b := strings.TrimSpace(branch)
	if b == "" {
		return nil, fmt.Errorf("branch required")
	}

	args := []string{"merge"}
	if noEdit {
		args = append(args, "--no-edit")
	}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, b)

	txt, err := o.runGitWithTimeout(10*time.Minute, args...)
	if err != nil {
		return nil, err
	}

	curBranch, _ := o.branchName()
	headHash, _ := o.headHash()
	return &MergeOut{Branch: curBranch, MergedBranch: b, HeadHash: headHash, Text: txt}, nil
}

type RebaseOut struct {
	Branch   string `json:"branch"`
	Action   string `json:"action"`
	Upstream string `json:"upstream,omitempty"`
	Onto     string `json:"onto,omitempty"`
	HeadHash string `json:"head_hash,omitempty"`
	Text     string `json:"text"`
}

func (o *Ops) Rebase(action, upstream, onto, branch string) (*RebaseOut, error) {
	act := strings.ToLower(strings.TrimSpace(action))
	if act == "" {
		act = "start"
	}

	args := []string{"rebase"}
	switch act {
	case "continue":
		args = append(args, "--continue")
	case "abort":
		args = append(args, "--abort")
	case "skip":
		args = append(args, "--skip")
	case "start":
		up := strings.TrimSpace(upstream)
		if up == "" {
			return nil, fmt.Errorf("upstream required for start")
		}
		if strings.TrimSpace(onto) != "" {
			args = append(args, "--onto", strings.TrimSpace(onto))
		}
		args = append(args, up)
		if strings.TrimSpace(branch) != "" {
			args = append(args, strings.TrimSpace(branch))
		}
	default:
		return nil, fmt.Errorf("unsupported action: %s", act)
	}

	txt, err := o.runGitWithTimeout(10*time.Minute, args...)
	if err != nil {
		return nil, err
	}

	curBranch, _ := o.branchName()
	headHash, _ := o.headHash()
	return &RebaseOut{
		Branch:   curBranch,
		Action:   act,
		Upstream: strings.TrimSpace(upstream),
		Onto:     strings.TrimSpace(onto),
		HeadHash: headHash,
		Text:     txt,
	}, nil
}

func (o *Ops) runGitWithTimeout(timeout time.Duration, args ...string) (string, error) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = o.Root
	out, err := cmd.CombinedOutput()
	txt := strings.TrimRight(string(out), "\r\n")
	if ctx.Err() == context.DeadlineExceeded {
		return txt, fmt.Errorf("git timeout")
	}
	if err != nil {
		if strings.TrimSpace(txt) != "" {
			return txt, fmt.Errorf("%v: %s", err, txt)
		}
		return txt, err
	}
	return txt, nil
}

func (o *Ops) branchName() (string, error) {
	r, err := o.repo()
	if err != nil {
		return "", err
	}
	head, err := r.Head()
	if err != nil {
		return "", err
	}
	if head == nil {
		return "", nil
	}
	return strings.TrimPrefix(head.Name().String(), "refs/heads/"), nil
}

func (o *Ops) headHash() (string, error) {
	txt, err := o.runGitWithTimeout(10*time.Second, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(txt), nil
}

func parseGitLogEntries(txt string) []LogEntry {
	lines := strings.Split(strings.ReplaceAll(txt, "\r\n", "\n"), "\n")
	var out []LogEntry
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		hash, msg := extractHashAndMessage(line)
		if hash == "" {
			continue
		}
		out = append(out, LogEntry{Hash: hash, Message: msg})
	}
	return out
}

func extractHashAndMessage(line string) (hash string, message string) {
	fields := strings.Fields(line)
	for i, f := range fields {
		clean := strings.Trim(f, "*|\\/()")
		if isLikelyGitHash(clean) {
			hash = clean
			idx := strings.Index(line, f)
			if idx >= 0 {
				rest := strings.TrimSpace(line[idx+len(f):])
				if rest == "" && i+1 < len(fields) {
					rest = strings.Join(fields[i+1:], " ")
				}
				message = rest
			}
			return
		}
	}
	return "", ""
}

func isLikelyGitHash(s string) bool {
	if len(s) < 7 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

func parseGitStashList(txt string) []StashEntry {
	lines := strings.Split(strings.ReplaceAll(txt, "\r\n", "\n"), "\n")
	var out []StashEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ref := ""
		idx := -1
		if strings.HasPrefix(line, "stash@{") {
			end := strings.Index(line, "}:")
			if end > 0 {
				ref = line[:end+1]
				rawIdx := strings.TrimSuffix(strings.TrimPrefix(ref, "stash@{"), "}")
				if n, e := parseInt(rawIdx); e == nil {
					idx = n
				}
			}
		}
		branch := ""
		msg := ""
		rest := line
		if ref != "" {
			rest = strings.TrimSpace(strings.TrimPrefix(line, ref+":"))
		}
		if strings.HasPrefix(rest, "On ") {
			tmp := strings.TrimPrefix(rest, "On ")
			parts := strings.SplitN(tmp, ":", 2)
			branch = strings.TrimSpace(parts[0])
			if len(parts) == 2 {
				msg = strings.TrimSpace(parts[1])
			} else {
				msg = strings.TrimSpace(tmp)
			}
		} else if strings.HasPrefix(strings.ToLower(rest), "wip on ") {
			tmp := rest[len("WIP on "):]
			parts := strings.SplitN(tmp, ":", 2)
			branch = strings.TrimSpace(parts[0])
			if len(parts) == 2 {
				msg = strings.TrimSpace(parts[1])
			} else {
				msg = strings.TrimSpace(tmp)
			}
		} else {
			msg = strings.TrimSpace(rest)
		}
		out = append(out, StashEntry{Index: idx, Ref: ref, Branch: branch, Message: msg, Raw: line})
	}
	return out
}

func parseInt(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not int")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
