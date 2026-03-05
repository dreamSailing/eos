package clip

import (
	"errors"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

var initOnce sync.Once
var initErr error

func initClipboard() error {
	initOnce.Do(func() {
		initErr = clipboard.Init()
	})
	return initErr
}

func ReadImage() ([]byte, error) {
	if err := initClipboard(); err != nil {
		return nil, err
	}
	for i := 0; i < 3; i++ {
		b := clipboard.Read(clipboard.FmtImage)
		if len(b) > 0 {
			return b, nil
		}
		if b2, err := readImageFallback(); err == nil && len(b2) > 0 {
			return b2, nil
		}
		time.Sleep(60 * time.Millisecond)
	}
	return nil, errors.New("empty clipboard image")
}
