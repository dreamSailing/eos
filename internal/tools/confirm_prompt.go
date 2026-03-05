package tools

import (
	"context"
)

type UserConfirmRequest struct {
	Title     string
	Question  string
	Options   []string
	AllowText bool
	TextHint  string
}

type UserConfirmResponse struct {
	Confirmed   bool
	Option      string
	OptionIndex int
	Text        string
}

var UserConfirmPrompt func(ctx context.Context, req UserConfirmRequest) (UserConfirmResponse, error)

