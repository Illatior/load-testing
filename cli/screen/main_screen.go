package screen

import (
	"context"
	"fmt"
	"github.com/illatior/nora/lib/metric"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/text"
	"sort"
	"time"
)

const (
	mainScreenLineChartId = "mainScreen-lineChart"
	maxInt64              = ^int64(0)
	minInt64              = -maxInt64 - 1
)

type mainScreen struct {
	displayInterval time.Duration

	opts screenOpts

	// header
	// `empty`

	// body
	latencyChart *linechart.LineChart

	// footer
	optionsText   *text.Text
	latenciesText *text.Text
	responsesText *text.Text
	errorsText    *text.Text
	errorCounts   *barchart.BarChart

	metricsCh chan metric.Metrics
	// todo add information for debug like duration, requests, responses, throughput, ...
}

func NewMainScreen() (Screen, error) {
	m, err := buildMainScreen()
	if err != nil {
		return nil, err
	}

	body := grid.RowHeightPerc(
		65,
		grid.Widget(m.latencyChart, borderLight(), borderTitle("Latency (ms)")),
	)

	footer := grid.RowHeightPerc(
		35,
		grid.ColWidthPerc(10, grid.Widget(m.optionsText, borderLight(), borderTitle("Options"))),
		grid.ColWidthPerc(20, grid.Widget(m.latenciesText, borderLight(), borderTitle("Latencies"))),
		grid.ColWidthPerc(20, grid.Widget(m.responsesText, borderLight(), borderTitle("Responses"))),
		grid.ColWidthPerc(20, grid.Widget(m.errorsText, borderLight(), borderTitle("Errors"))),
		grid.ColWidthPerc(30, grid.Widget(m.errorCounts, borderLight(), borderTitle("Errors count"))),
	)

	m.opts = screenOpts{
		header: nil,
		body:   body,
		footer: footer,
	}

	return m, nil
}

func buildMainScreen() (*mainScreen, error) {
	latencyChart, err := newLineChart()
	if err != nil {
		return nil, err
	}

	optionsText, err := newTextBlock()
	if err != nil {
		return nil, err
	}
	latenciesText, err := newTextBlock()
	if err != nil {
		return nil, err
	}
	responsesText, err := newTextBlock()
	if err != nil {
		return nil, err
	}
	errorsText, err := newTextBlock()
	if err != nil {
		return nil, err
	}
	errorsCount, err := newBarChart()
	if err != nil {
		return nil, err
	}

	return &mainScreen{
		displayInterval: 30 * time.Second,

		latencyChart:  latencyChart,
		optionsText:   optionsText,
		latenciesText: latenciesText,
		responsesText: responsesText,
		errorsText:    errorsText,
		errorCounts:   errorsCount,

		metricsCh: make(chan metric.Metrics),
	}, nil
}

func (s *mainScreen) GetBody() grid.Element {
	return s.opts.body
}

func (s *mainScreen) GetHeader() grid.Element {
	return s.opts.header
}

func (s *mainScreen) GetFooter() grid.Element {
	return s.opts.footer
}

func (s *mainScreen) GetMetricsChan() chan<- metric.Metrics {
	return s.metricsCh
}

func (s *mainScreen) ChangeDisplayInterval(t time.Duration) {
	s.displayInterval += t
	if s.displayInterval <= 0 {
		s.displayInterval = time.Second
	}
}

func (s *mainScreen) Run(ctx context.Context) {
	defer close(s.metricsCh)

	for {
		select {
		case <-ctx.Done():
			return
		case m := <-s.metricsCh:
			//latencyMetrics := m.GetLatencyMetrics()

			executionStatistic := m.GetExecutionStatistic()
			s.updateWithExecutionStatistic(executionStatistic)

			chartMetrics := m.GetChartMetrics()
			s.updateWithChartMetrics(chartMetrics)

			latencyMetrics := m.GetLatencyMetrics()
			s.updateWithLatencyMetrics(latencyMetrics)
		}
	}
}

const (
	responsesPattern = `Total: %v
Success: %v
Error: %v`

	errorsPattern = "%v: %v\n"
)

func (s *mainScreen) updateWithExecutionStatistic(es metric.ExecutionStatistic) {
	// update responses text
	total := es.GetTotalExecuted()
	success := es.GetTotalSuccess()
	s.responsesText.Write(
		fmt.Sprintf(responsesPattern, total, success, total-success),
		text.WriteReplace(),
	)

	errors := es.GetErrors()
	sort.Strings(errors)

	errorsText := ""
	var errorCounts []int
	maxCount := 0
	// update errors text
	for _, err := range errors {
		errorsCount := es.GetErrorsCount(err)

		errorsText += fmt.Sprintf(errorsPattern, err, errorsCount)
		errorCounts = append(errorCounts, int(errorsCount))
		if int(errorsCount) > maxCount {
			maxCount = int(errorsCount)
		}
	}
	s.errorsText.Write(
		errorsText,
		text.WriteReplace(),
	)

	// update error counts
	s.errorCounts.Values(
		errorCounts,
		maxCount,
	)
}

func (s *mainScreen) updateWithChartMetrics(cm metric.ChartMetrics) {
	to := time.Now()
	from := to.Add(-s.displayInterval)

	r := cm.GetInRange(from, to)

	var values []float64
	for _, v := range r {
		values = append(values, float64(v.Duration/time.Millisecond))
	}
	s.latencyChart.Series(
		"Request duration (in ms)",
		values,
	)
}

const (
	latenciesPattern = `Avg: %v
Min: %v
Max: %v

Q1:     %v
Median: %v
Q3:     %v
P90: %v
P95: %v
P99: %v`
)

func (s *mainScreen) updateWithLatencyMetrics(lm metric.LatencyMetrics) {
	s.latenciesText.Write(
		fmt.Sprintf(latenciesPattern,
			lm.GetAvg(), lm.GetMin(), lm.GetMax(),
			lm.GetPercentile(25), lm.GetPercentile(50), lm.GetPercentile(75),
			lm.GetPercentile(90), lm.GetPercentile(95), lm.GetPercentile(99)),
		text.WriteReplace(),
	)
}
