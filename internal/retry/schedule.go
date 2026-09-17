package retry

import(
	"time"
)

var retryIntervals = []time.Duration {
	24 * time.Hour,  // Day 1
	48 * time.Hour,  // Day 3
	96 * time.Hour,	 // Day 7
}

func nextRetryDelay(currentRetryCount int) (time.Duration, bool) {
	if currentRetryCount >= len(retryIntervals) {
		return 0, false
	}
	return retryIntervals[currentRetryCount], true
}
