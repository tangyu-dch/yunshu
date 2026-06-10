package fsesl

import (
	"errors"
	"testing"
)

func TestIsConnectionWriteError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "broken pipe", err: errors.New("write tcp 127.0.0.1:1->127.0.0.1:2: write: broken pipe"), want: true},
		{name: "connection reset", err: errors.New("read: connection reset by peer"), want: true},
		{name: "closed network connection", err: errors.New("use of closed network connection"), want: true},
		{name: "eof", err: errors.New("EOF"), want: true},
		{name: "other", err: errors.New("freeswitch node not configured"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isConnectionWriteError(tc.err); got != tc.want {
				t.Fatalf("isConnectionWriteError()=%v want %v", got, tc.want)
			}
		})
	}
}
