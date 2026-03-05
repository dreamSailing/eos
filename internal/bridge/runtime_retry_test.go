package bridge

import (
	"context"
	"errors"
	"testing"
)

func TestIsRetryableModelError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{name: "429", err: errors.New("429 Too Many Requests"), want: true},
		{name: "502", err: errors.New("502 Bad Gateway"), want: true},
		{name: "conn_reset", err: errors.New("connection reset by peer"), want: true},
		{name: "401", err: errors.New("401 Unauthorized"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableModelError(tc.err); got != tc.want {
				t.Fatalf("got %v want %v (err=%v)", got, tc.want, tc.err)
			}
		})
	}
}
