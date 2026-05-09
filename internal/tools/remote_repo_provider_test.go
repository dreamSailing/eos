package tools

import "testing"

func TestParseRemoteRepoURLHTTPS(t *testing.T) {
	ref, err := parseRemoteRepoURL("github", "https://github.com/acme/demo.git")
	if err != nil {
		t.Fatalf("parseRemoteRepoURL error: %v", err)
	}
	if ref.Owner != "acme" || ref.Repo != "demo" {
		t.Fatalf("unexpected repo ref: %+v", ref)
	}
	if ref.WebURL != "https://github.com/acme/demo" {
		t.Fatalf("unexpected web url: %s", ref.WebURL)
	}
}

func TestParseRemoteRepoURLSSH(t *testing.T) {
	ref, err := parseRemoteRepoURL("gitee", "git@gitee.com:acme/demo.git")
	if err != nil {
		t.Fatalf("parseRemoteRepoURL error: %v", err)
	}
	if ref.Host != "gitee.com" || ref.Owner != "acme" || ref.Repo != "demo" {
		t.Fatalf("unexpected repo ref: %+v", ref)
	}
}
