// Package srs implements a plain 5-box Leitner scheduler.
//
// Not a full SM2: no ease factor, no per-card interval tuning. For learning
// vocabulary from a book that's plenty — the thing that matters is that a
// missed word comes back tomorrow and a known one comes back rarely.
package srs

import "time"

const MaxBox = 5

// intervals[box-1] = days until the next review once a card sits in that box.
var intervals = [MaxBox]int{1, 3, 7, 14, 30}

type Result string

const (
	Again Result = "again"
	Good  Result = "good"
)

// Next returns the box and next-review time after answering a card currently
// in `box` (1-based) with `result`, relative to `now`.
func Next(box int, result Result, now time.Time) (nextBox int, nextReview time.Time) {
	if box < 1 {
		box = 1
	}
	switch result {
	case Good:
		nextBox = box + 1
		if nextBox > MaxBox {
			nextBox = MaxBox
		}
	default: // Again, or anything unrecognized — fail safe to the start
		nextBox = 1
	}
	days := intervals[nextBox-1]
	return nextBox, now.AddDate(0, 0, days)
}
