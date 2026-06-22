// Package queue provides a Redis-backed job queue for background work
// (currently: diagram screenshot generation).
package queue

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const screenshotKey = "screenshot:jobs"

// debounceWindow delays a job until this long after the most recent content
// change. A burst of updates to one diagram keeps pushing the due time forward,
// so the diagram is screenshotted once, after edits settle.
const debounceWindow = 3 * time.Second

// ScreenshotJob identifies a diagram whose preview thumbnail must be regenerated.
type ScreenshotJob struct {
	OrgID     string `json:"orgId"`
	DiagramID string `json:"diagramId"`
}

// Queue is a Redis sorted-set job queue. The set is keyed by diagram so repeated
// enqueues of the same diagram coalesce into a single pending entry (latest wins);
// the score is the time the job becomes due (debounced).
type Queue struct {
	rc *redis.Client
}

// New creates a Queue from a redis:// or rediss:// URL.
func New(redisURL string) (*Queue, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Queue{rc: redis.NewClient(opts)}, nil
}

func memberOf(job ScreenshotJob) string {
	return job.OrgID + "|" + job.DiagramID
}

func jobOf(member string) (ScreenshotJob, bool) {
	orgID, diagramID, ok := strings.Cut(member, "|")
	if !ok {
		return ScreenshotJob{}, false
	}
	return ScreenshotJob{OrgID: orgID, DiagramID: diagramID}, true
}

// EnqueueScreenshot schedules (or reschedules) a screenshot for a diagram. If a
// job for the same diagram is already pending, its due time is reset, collapsing
// rapid successive updates into one screenshot.
func (q *Queue) EnqueueScreenshot(ctx context.Context, job ScreenshotJob) error {
	due := float64(time.Now().Add(debounceWindow).UnixMilli())
	return q.rc.ZAdd(ctx, screenshotKey, redis.Z{Score: due, Member: memberOf(job)}).Err()
}

// claimDue atomically takes the single earliest job whose due time has passed,
// so a job is never handed out before its debounce window elapses or claimed twice.
var claimDue = redis.NewScript(`
	local due = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, 1)
	if #due == 0 then return false end
	redis.call('ZREM', KEYS[1], due[1])
	return due[1]
`)

// DequeueScreenshot claims the next due job. It returns (job, false, nil) when no
// job is currently due (the caller should poll again shortly).
func (q *Queue) DequeueScreenshot(ctx context.Context) (ScreenshotJob, bool, error) {
	now := float64(time.Now().UnixMilli())
	res, err := claimDue.Run(ctx, q.rc, []string{screenshotKey}, now).Result()
	if err == redis.Nil {
		return ScreenshotJob{}, false, nil
	}
	if err != nil {
		return ScreenshotJob{}, false, err
	}
	member, ok := res.(string)
	if !ok {
		return ScreenshotJob{}, false, nil
	}
	job, ok := jobOf(member)
	if !ok {
		return ScreenshotJob{}, false, nil
	}
	return job, true, nil
}
