package main

import (
	"time"

	"github.com/djherbis/times"
)

// timesForPath is overridden in tests to supply deterministic times without
// depending on real filesystem/platform birth-time behavior.
var timesForPath = fileTimes

// fileTimes returns a file's modified time and its best-available creation
// ("birth") time, falling back to modified time on filesystems that don't
// track birth time (e.g. most Linux filesystems).
func fileTimes(path string) (mod, birth time.Time, err error) {
	t, err := times.Stat(path)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	mod = t.ModTime()
	if t.HasBirthTime() {
		birth = t.BirthTime()
	} else {
		birth = mod
	}
	return mod, birth, nil
}
