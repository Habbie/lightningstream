package utils

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SleepContext sleeps for given duration. If the context closes in the
// meantime, it returns immediately with a context.Canceled error.
func SleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return context.Canceled
	case <-t.C:
		return nil
	}
}

// IsCanceled checks if the context has been canceled.
func IsCanceled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// DisplayASCII represents a key as ascii if it only contains safe ascii characters.
// If it contains unsafe characters, these are replaced by '.' and a hex
// representation is added to the output.
func DisplayASCII(b []byte) string {
	ret := make([]byte, len(b))
	unsafe := false
	for i, ch := range b {
		if ch < 32 || ch > 126 {
			ret[i] = '.'
			unsafe = true
		} else {
			ret[i] = ch
		}
	}
	if unsafe {
		return fmt.Sprintf("%s [% 0x]", string(ret), b)
	}
	return string(ret)
}

// Cut cuts s around the first instance of sep,
// returning the text before and after sep.
// The found result reports whether sep appears in s.
// If sep does not appear in s, cut returns s, "", false.
//
// This is a copy of strings.Cut from Go 1.18,
// see https://github.com/golang/go/issues/46336
// TODO: remove when we switch to Go 1.18 and use strings.Cut
func Cut(s, sep string) (before, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

func TimeDiff(t1, t0 time.Time) time.Duration {
	return t1.Sub(t0).Round(time.Millisecond)
}
