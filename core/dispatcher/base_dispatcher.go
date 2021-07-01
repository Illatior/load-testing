package dispatcher

import (
	"context"
	"github.com/illatior/load-testing/core/executor"
	"github.com/illatior/load-testing/core/metric"
	"sync"
	"time"
)

type baseDispatcher struct {
	workers uint64
}

func NewDispatcher(workers uint64) Dispatcher {
	return &baseDispatcher{
		workers: workers,
	}
}

func (d *baseDispatcher) Dispatch(ctx context.Context, scheduler Scheduler, executor executor.Executor, duration time.Duration) <-chan *metric.Result {
	var wg sync.WaitGroup
	var doneCtx, cancel = context.WithCancel(ctx)
	done := make(chan bool, 0)

	ticks := make(chan interface{})
	results := make(chan *metric.Result)

	for i := uint64(0); i < d.workers; i++ {
		wg.Add(1)
		go executor.ScheduleExecution(doneCtx, &wg, ticks, results)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		select {
		case <-ctx.Done():
			return
		case <-done:
			for {
				if len(ticks) == 0 {
					return
				}

				time.Sleep(time.Millisecond * 50)
			}
		}
	}()

	go func() {
		defer close(results)
		defer close(ticks)
		defer wg.Wait()
		defer func() { done <- true }()

		lastExecution, executed := time.Now(), uint64(0)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				break
			}

			timeElapsed := time.Since(lastExecution)
			if timeElapsed > duration && duration != 0 {
				return
			}

			next, stop := scheduler.ScheduleNextExecution(timeElapsed, executed)
			if stop {
				return
			}
			time.Sleep(next)

			ticks <- struct{}{}
			executed++
		}
	}()

	return results
}
