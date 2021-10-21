package task_scheduler

import (
	"context"
	"github.com/illatior/task-scheduler/core"
	"github.com/illatior/task-scheduler/core/executor"
	"github.com/illatior/task-scheduler/core/metric"
	"github.com/illatior/task-scheduler/core/scheduler"
	"github.com/illatior/task-scheduler/cui"
	"golang.org/x/sync/errgroup"
	"runtime"
	"sync"
	"time"
)

type taskScheduler struct {
	duration time.Duration

	executorsCount int
	sch            scheduler.Scheduler
	exec           executor.Executor

	c cui.ConsoleUserInterface
}

func New(opts ...Option) (*taskScheduler, error) {
	sch := scheduler.ConstantScheduler{
		Frequency: 10,
		Period:    1 * time.Second,
	}
	exec := executor.New()

	ts := &taskScheduler{
		duration:       10 * time.Second,
		sch:            sch,
		exec:           exec,
		executorsCount: runtime.GOMAXPROCS(0),
		c:              nil,
	}

	for _, opt := range opts {
		err := opt.apply(ts)
		if err != nil {
			return nil, err
		}
	}

	return ts, nil
}

//func (ts *taskScheduler) Run(ctx context.Context) <-chan
//
//func (ts *taskScheduler) RunWithRawResults(ctx context.Context) <-chan *metric.Result {
//
//}

func (ts *taskScheduler) Run(ctx context.Context) <-chan *metric.Result {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)

	res := core.Dispatch(ctx, ts.sch, ts.exec, ts.duration, ts.executorsCount)

	// TODO add errs chan with exiting after receiving any error and replace errgroup with it
	userRes := make(chan *metric.Result)
	uiRes := make(chan *metric.Result)
	go func() {
		defer close(userRes)
		defer close(uiRes)
		defer eg.Wait()
		defer cancel()

		dispatchDone := make(chan bool, 1)
		cuiDone := make(chan bool, 1)

		eg.Go(func() error {
			return runMetricRepeater(ctx, userRes, uiRes, res, dispatchDone)
		})

		runCiFunc := ts.getRunCuiFunc(ctx, uiRes, cuiDone, dispatchDone)
		eg.Go(func() error {
			return runCiFunc()
		})

		select {
		case <-ctx.Done():
			return
		case <-cuiDone:
			return
		}
	}()

	return userRes
}

func runMetricRepeater(ctx context.Context,
	userCh, uiCh chan<- *metric.Result,
	resCh <-chan *metric.Result,
	done chan<- bool) error {
	defer func() {
		done <- true
	}()

	// TODO find a better solution to duplicate execution results (and also with ctx.Done handling)
	for m := range resCh {
		userCh <- m
		uiCh <- m
	}
	return nil
}

func (ts *taskScheduler) getRunCuiFunc(ctx context.Context,
	ch <-chan *metric.Result,
	cuiDone chan<- bool,
	dispatchDone <-chan bool) func() error {
	if ts.c != nil {
		return func() error {
			return ts.runCui(ctx, ch, cuiDone)
		}
	}

	return func() error {
		defer func() {
			cuiDone <- true
		}()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-dispatchDone:
				return nil
			case <-ch:
				continue
			}
		}
	}
}

// runCui method is blocking
func (ts *taskScheduler) runCui(ctx context.Context, res <-chan *metric.Result, done chan<- bool) error {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer wg.Wait()
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case m := <-res:
				if m == nil {
					continue
				}

				ts.c.AcceptMetric(m)
			}
		}
	}()

	return ts.c.Run(ctx, done)
}
