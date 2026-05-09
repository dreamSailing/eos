package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
)

type remoteRepoRef struct {
	Platform string `json:"platform"`
	Host     string `json:"host"`
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	CloneURL string `json:"clone_url"`
	WebURL   string `json:"web_url"`
}

type remoteAccount struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

type remotePullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type remoteAuthorization struct {
	Status string         `json:"status"`
	Data   map[string]any `json:"data,omitempty"`
}

type remoteProvider interface {
	Platform() string
	NormalizeRepoURL(repoURL string) (remoteRepoRef, error)
	CurrentUser(ctx context.Context, token string) (remoteAccount, error)
	ResolveToken(ctx context.Context, cfg *config.Config, params map[string]any) (config.RemoteAuthToken, *remoteAuthorization, error)
	AuthUsername(auth config.RemoteAuthToken) string
	CreatePullRequest(ctx context.Context, token string, ref remoteRepoRef, title, body, base, head string) (remotePullRequest, error)
}

func remoteProviderFor(platform string) (remoteProvider, error) {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case string(config.RemotePlatformGitHub):
		return githubRemoteProvider{}, nil
	case string(config.RemotePlatformGitee):
		return giteeRemoteProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported remote platform: %s", strings.TrimSpace(platform))
	}
}

func remoteHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func remoteDoJSON(ctx context.Context, method, endpoint string, headers map[string]string, form url.Values, body any, out any) error {
	var reqBody io.Reader
	if form != nil {
		reqBody = strings.NewReader(form.Encode())
	}
	if form == nil && body != nil {
		buf := bytes.NewBuffer(nil)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return err
		}
		reqBody = buf
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "eos-remote-repo/1.0")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if form == nil && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := remoteHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(payload))
		if len(msg) > 240 {
			msg = msg[:240] + "..."
		}
		return fmt.Errorf("http %d: %s", resp.StatusCode, msg)
	}
	if out == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}

func remoteRandomState(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(b)
}

func trimDotGit(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimSuffix(repo, ".git")
	return repo
}

func parseRemoteRepoURL(platform, repoURL string) (remoteRepoRef, error) {
	raw := strings.TrimSpace(repoURL)
	if raw == "" {
		return remoteRepoRef{}, fmt.Errorf("repo_url required")
	}
	ref := remoteRepoRef{Platform: strings.ToLower(strings.TrimSpace(platform)), CloneURL: raw}
	if strings.HasPrefix(raw, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(raw, "git@"), ":", 2)
		if len(parts) != 2 {
			return remoteRepoRef{}, fmt.Errorf("invalid ssh repo url: %s", raw)
		}
		ref.Host = strings.ToLower(strings.TrimSpace(parts[0]))
		path := strings.Trim(strings.TrimSpace(parts[1]), "/")
		segs := strings.Split(path, "/")
		if len(segs) < 2 {
			return remoteRepoRef{}, fmt.Errorf("invalid repo path: %s", raw)
		}
		ref.Owner = segs[0]
		ref.Repo = trimDotGit(segs[1])
		ref.WebURL = fmt.Sprintf("https://%s/%s/%s", ref.Host, ref.Owner, ref.Repo)
		return ref, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return remoteRepoRef{}, err
	}
	ref.Host = strings.ToLower(strings.TrimSpace(u.Host))
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) < 2 {
		return remoteRepoRef{}, fmt.Errorf("invalid repo url: %s", raw)
	}
	ref.Owner = segs[0]
	ref.Repo = trimDotGit(segs[1])
	ref.WebURL = fmt.Sprintf("https://%s/%s/%s", ref.Host, ref.Owner, ref.Repo)
	if ref.CloneURL == "" {
		ref.CloneURL = ref.WebURL + ".git"
	}
	return ref, nil
}

func remoteBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.ToSlash(filepath.Join(".eos", "remotes"))
	}
	return filepath.ToSlash(filepath.Join(home, ".eos", "remotes"))
}

func remoteRepoDir(ref remoteRepoRef) string {
	return filepath.ToSlash(filepath.Join(remoteBaseDir(), ref.Platform, ref.Owner, ref.Repo))
}

func loadRemoteConfig() (config.Config, string) {
	return config.Load()
}

func saveRemoteConfig(cfg config.Config, path string) error {
	if path == "" {
		path = config.Path()
	}
	return config.Save(cfg, path)
}

func ensureRemoteMaps(cfg *config.Config) {
	if cfg.RemoteProviders == nil {
		cfg.RemoteProviders = map[string]config.RemoteProviderConfig{}
	}
	if cfg.RemoteAuth == nil {
		cfg.RemoteAuth = map[string]config.RemoteAuthToken{}
	}
}

func upsertRemoteRepo(cfg *config.Config, entry config.RemoteRepoEntry) {
	for i := range cfg.RemoteRepos {
		if strings.EqualFold(cfg.RemoteRepos[i].RepoURL, entry.RepoURL) {
			cfg.RemoteRepos[i] = entry
			return
		}
	}
	cfg.RemoteRepos = append(cfg.RemoteRepos, entry)
}

type githubRemoteProvider struct{}

func (githubRemoteProvider) Platform() string { return string(config.RemotePlatformGitHub) }

func (githubRemoteProvider) NormalizeRepoURL(repoURL string) (remoteRepoRef, error) {
	ref, err := parseRemoteRepoURL(string(config.RemotePlatformGitHub), repoURL)
	if err != nil {
		return remoteRepoRef{}, err
	}
	if ref.Host == "" {
		ref.Host = "github.com"
	}
	return ref, nil
}

func (githubRemoteProvider) CurrentUser(ctx context.Context, token string) (remoteAccount, error) {
	var out struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	err := remoteDoJSON(ctx, http.MethodGet, "https://api.github.com/user", map[string]string{
		"Accept":        "application/vnd.github+json",
		"Authorization": "Bearer " + strings.TrimSpace(token),
	}, nil, nil, &out)
	if err != nil {
		return remoteAccount{}, err
	}
	return remoteAccount{ID: fmt.Sprintf("%d", out.ID), Login: out.Login, Name: out.Name}, nil
}

func (githubRemoteProvider) ResolveToken(ctx context.Context, cfg *config.Config, params map[string]any) (config.RemoteAuthToken, *remoteAuthorization, error) {
	ensureRemoteMaps(cfg)
	plat := string(config.RemotePlatformGitHub)
	if auth, ok := cfg.RemoteAuth[plat]; ok && strings.TrimSpace(auth.AccessToken) != "" {
		return auth, nil, nil
	}
	if providerCfg, ok := cfg.RemoteProviders[plat]; ok && strings.TrimSpace(providerCfg.AccessToken) != "" {
		auth := config.RemoteAuthToken{
			Platform:    config.RemotePlatformGitHub,
			AccessToken: strings.TrimSpace(providerCfg.AccessToken),
			Login:       strings.TrimSpace(providerCfg.Username),
		}
		return auth, nil, nil
	}
	clientID := strings.TrimSpace(cfg.RemoteProviders[plat].OAuth.ClientID)
	clientSecret := strings.TrimSpace(cfg.RemoteProviders[plat].OAuth.ClientSecret)
	deviceCode, _ := params["device_code"].(string)
	if clientID == "" {
		return config.RemoteAuthToken{}, nil, fmt.Errorf("github 未配置 access_token 或 oauth.client_id")
	}
	if strings.TrimSpace(deviceCode) == "" {
		var out struct {
			DeviceCode      string `json:"device_code"`
			UserCode        string `json:"user_code"`
			VerificationURI string `json:"verification_uri"`
			ExpiresIn       int    `json:"expires_in"`
			Interval        int    `json:"interval"`
		}
		if err := remoteDoJSON(ctx, http.MethodPost, "https://github.com/login/device/code", map[string]string{
			"Accept": "application/json",
		}, url.Values{
			"client_id": []string{clientID},
			"scope":     []string{"repo"},
		}, nil, &out); err != nil {
			return config.RemoteAuthToken{}, nil, err
		}
		return config.RemoteAuthToken{}, &remoteAuthorization{
			Status: "authorization_required",
			Data: map[string]any{
				"platform":         plat,
				"flow":             "device_code",
				"device_code":      out.DeviceCode,
				"user_code":        out.UserCode,
				"verification_uri": out.VerificationURI,
				"expires_in":       out.ExpiresIn,
				"interval":         out.Interval,
				"message":          "请在浏览器中完成 GitHub 授权，然后把 device_code 继续传给 remote_repo_connect 轮询结果。",
			},
		}, nil
	}
	var tokenRes struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}
	form := url.Values{
		"client_id":   []string{clientID},
		"device_code": []string{strings.TrimSpace(deviceCode)},
		"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	if err := remoteDoJSON(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", map[string]string{
		"Accept": "application/json",
	}, form, nil, &tokenRes); err != nil {
		return config.RemoteAuthToken{}, nil, err
	}
	if strings.TrimSpace(tokenRes.AccessToken) == "" {
		return config.RemoteAuthToken{}, &remoteAuthorization{
			Status: "authorization_pending",
			Data: map[string]any{
				"platform":       plat,
				"flow":           "device_code",
				"device_code":    strings.TrimSpace(deviceCode),
				"provider_error": strings.TrimSpace(tokenRes.Error),
			},
		}, nil
	}
	return config.RemoteAuthToken{
		Platform:    config.RemotePlatformGitHub,
		AccessToken: strings.TrimSpace(tokenRes.AccessToken),
		TokenType:   strings.TrimSpace(tokenRes.TokenType),
		Scope:       strings.TrimSpace(tokenRes.Scope),
	}, nil, nil
}

func (githubRemoteProvider) AuthUsername(auth config.RemoteAuthToken) string {
	if strings.TrimSpace(auth.Login) != "" {
		return strings.TrimSpace(auth.Login)
	}
	return "oauth2"
}

func (githubRemoteProvider) CreatePullRequest(ctx context.Context, token string, ref remoteRepoRef, title, body, base, head string) (remotePullRequest, error) {
	var out struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	err := remoteDoJSON(ctx, http.MethodPost, fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", ref.Owner, ref.Repo), map[string]string{
		"Accept":        "application/vnd.github+json",
		"Authorization": "Bearer " + strings.TrimSpace(token),
	}, nil, map[string]any{
		"title": title,
		"body":  body,
		"base":  base,
		"head":  head,
	}, &out)
	if err != nil {
		return remotePullRequest{}, err
	}
	return remotePullRequest{Number: out.Number, URL: out.HTMLURL}, nil
}

type giteeRemoteProvider struct{}

func (giteeRemoteProvider) Platform() string { return string(config.RemotePlatformGitee) }

func (giteeRemoteProvider) NormalizeRepoURL(repoURL string) (remoteRepoRef, error) {
	ref, err := parseRemoteRepoURL(string(config.RemotePlatformGitee), repoURL)
	if err != nil {
		return remoteRepoRef{}, err
	}
	if ref.Host == "" {
		ref.Host = "gitee.com"
	}
	return ref, nil
}

func (giteeRemoteProvider) CurrentUser(ctx context.Context, token string) (remoteAccount, error) {
	var out struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	endpoint := "https://gitee.com/api/v5/user?access_token=" + url.QueryEscape(strings.TrimSpace(token))
	if err := remoteDoJSON(ctx, http.MethodGet, endpoint, nil, nil, nil, &out); err != nil {
		return remoteAccount{}, err
	}
	return remoteAccount{ID: fmt.Sprintf("%d", out.ID), Login: out.Login, Name: out.Name}, nil
}

func (giteeRemoteProvider) ResolveToken(ctx context.Context, cfg *config.Config, params map[string]any) (config.RemoteAuthToken, *remoteAuthorization, error) {
	ensureRemoteMaps(cfg)
	plat := string(config.RemotePlatformGitee)
	if auth, ok := cfg.RemoteAuth[plat]; ok && strings.TrimSpace(auth.AccessToken) != "" {
		return auth, nil, nil
	}
	if providerCfg, ok := cfg.RemoteProviders[plat]; ok && strings.TrimSpace(providerCfg.AccessToken) != "" {
		auth := config.RemoteAuthToken{
			Platform:    config.RemotePlatformGitee,
			AccessToken: strings.TrimSpace(providerCfg.AccessToken),
			Login:       strings.TrimSpace(providerCfg.Username),
		}
		return auth, nil, nil
	}
	app := cfg.RemoteProviders[plat].OAuth
	clientID := strings.TrimSpace(app.ClientID)
	clientSecret := strings.TrimSpace(app.ClientSecret)
	redirectURI := strings.TrimSpace(app.RedirectURI)
	code, _ := params["authorization_code"].(string)
	state, _ := params["state"].(string)
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		return config.RemoteAuthToken{}, nil, fmt.Errorf("gitee 未配置完整 oauth.client_id / client_secret / redirect_uri")
	}
	if strings.TrimSpace(code) == "" {
		if strings.TrimSpace(state) == "" {
			state = remoteRandomState("gitee")
		}
		authURL := "https://gitee.com/oauth/authorize?" + url.Values{
			"client_id":     []string{clientID},
			"redirect_uri":  []string{redirectURI},
			"response_type": []string{"code"},
			"state":         []string{state},
			"scope":         []string{"projects pull_requests"},
		}.Encode()
		return config.RemoteAuthToken{}, &remoteAuthorization{
			Status: "authorization_required",
			Data: map[string]any{
				"platform": plat,
				"flow":     "authorization_code",
				"state":    state,
				"auth_url": authURL,
				"message":  "请完成 Gitee 授权后，把回调中的 code 继续传给 remote_repo_connect。",
			},
		}, nil
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		CreatedAt    int64  `json:"created_at"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := remoteDoJSON(ctx, http.MethodPost, "https://gitee.com/oauth/token", nil, url.Values{
		"grant_type":    []string{"authorization_code"},
		"code":          []string{strings.TrimSpace(code)},
		"client_id":     []string{clientID},
		"client_secret": []string{clientSecret},
		"redirect_uri":  []string{redirectURI},
	}, nil, &out); err != nil {
		return config.RemoteAuthToken{}, nil, err
	}
	return config.RemoteAuthToken{
		Platform:     config.RemotePlatformGitee,
		AccessToken:  strings.TrimSpace(out.AccessToken),
		RefreshToken: strings.TrimSpace(out.RefreshToken),
		TokenType:    strings.TrimSpace(out.TokenType),
		Scope:        strings.TrimSpace(out.Scope),
		ExpiryUnix:   out.CreatedAt + out.ExpiresIn,
	}, nil, nil
}

func (giteeRemoteProvider) AuthUsername(auth config.RemoteAuthToken) string {
	if strings.TrimSpace(auth.Login) != "" {
		return strings.TrimSpace(auth.Login)
	}
	return "oauth2"
}

func (giteeRemoteProvider) CreatePullRequest(ctx context.Context, token string, ref remoteRepoRef, title, body, base, head string) (remotePullRequest, error) {
	var out struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
	}
	err := remoteDoJSON(ctx, http.MethodPost, fmt.Sprintf("https://gitee.com/api/v5/repos/%s/%s/pulls", ref.Owner, ref.Repo), nil, url.Values{
		"access_token": []string{strings.TrimSpace(token)},
		"title":        []string{title},
		"body":         []string{body},
		"base":         []string{base},
		"head":         []string{head},
	}, nil, &out)
	if err != nil {
		return remotePullRequest{}, err
	}
	return remotePullRequest{Number: out.Number, URL: out.URL}, nil
}
