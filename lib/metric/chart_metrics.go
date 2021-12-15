package metric

import (
	"fmt"
	"time"
)

type chartMetrics struct {
	entries []*ChartEntry
}

func NewChartMetrics() ChartMetrics {
	return &chartMetrics{
		entries: make([]*ChartEntry, 0),
	}
}

func (cm *chartMetrics) ConsumeResult(res *Result) {
	entry := &ChartEntry{
		Timestamp: res.Start, // TODO prob res.Start.Add(res.Duration) will be more correct
		Duration:  res.Duration,
	}

	cm.entries = append(cm.entries, entry)
}

func (cm *chartMetrics) GetInRange(from, to time.Time) []ChartEntry {
	res := make([]ChartEntry, 0)
	for i := 0; i < len(cm.entries); i++ {
		entry := *cm.entries[i]
		if entry.Timestamp.After(from) || entry.Timestamp.Before(to) {
			res = append(res, entry)
		}
	}

	return res
}

const (
	chartEntryPattern = "%v,%v\n"
)

func (cm *chartMetrics) String() string {
	var res string
	for _, entry := range cm.entries {
		res += fmt.Sprintf(chartEntryPattern, entry.Timestamp, entry.Duration)
	}

	return res
}
