package notify

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/dreamSailing/eos/internal/pkg/utils"

	"github.com/gen2brain/beeep"
)

var once sync.Once

var defaultSender = func(title, message string) error {
	once.Do(func() {
		beeep.AppName = "EOS"
	})
	return beeep.Notify(title, message, "")
}

var sender = defaultSender

func SetSender(fn func(title, message string) error) {
	if fn == nil {
		return
	}
	sender = fn
}

func ResetSender() {
	sender = defaultSender
}

func NotifyAsync(title, message string) {
	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)
	if title == "" && message == "" {
		return
	}
	if title == "" {
		title = "EOS"
	}
	go func() {
		defer func() { _ = recover() }()
		if err := sender(title, message); err != nil {
			slog.Debug("notify.error", "component", utils.ComponentSystem, "err", err.Error())
		}
	}()
}
