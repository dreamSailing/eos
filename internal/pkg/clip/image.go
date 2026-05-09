package clip

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


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
