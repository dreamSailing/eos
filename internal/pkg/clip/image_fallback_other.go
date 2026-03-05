//go:build !windows

package clip

import "errors"

func readImageFallback() ([]byte, error) {
	return nil, errors.New("no fallback")
}

