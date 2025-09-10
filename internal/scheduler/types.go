package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
)

// JobInfo contains information about a scheduled job
type JobInfo struct {
	Name     string       `json:"name"`
	EntryID  cron.EntryID `json:"entry_id"`
	Next     time.Time    `json:"next"`
	Previous time.Time    `json:"previous"`
}
