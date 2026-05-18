package runtime

import (
	"fmt"
	"time"
)

func (d *duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", text, err)
	}
	d.Duration = dur
	return nil
}

func (d duration) val(def time.Duration) time.Duration {
	if d.Duration != 0 {
		return d.Duration
	}
	return def
}
