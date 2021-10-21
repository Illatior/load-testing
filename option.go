package task_scheduler

import (
	"context"
	"errors"
	"github.com/illatior/task-scheduler/core/executor"
	"github.com/illatior/task-scheduler/core/scheduler"
	"github.com/illatior/task-scheduler/core/task"
	"github.com/illatior/task-scheduler/cui"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/termbox"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"runtime"
	"time"
)

type Option interface {
	apply(ts *taskScheduler) error
}

type option func(ts *taskScheduler) error

func (o option) apply(ts *taskScheduler) error {
	return o(ts)
}

func WithDuration(d time.Duration) Option {
	return option(func(ts *taskScheduler) error {
		if d <= 0 {
			return errors.New("duration of scheduling must be > 0")
		}

		ts.duration = d
		return nil
	})
}

func WithScheduler(sch func() scheduler.Scheduler) Option {
	return option(func(ts *taskScheduler) error {
		ts.sch = sch()
		return nil
	})
}

func WithExecutor(exec func() executor.Executor) Option {
	return option(func(ts *taskScheduler) error {
		ts.exec = exec()
		return nil
	})
}

func WithExecutorsCount(c int) Option {
	return option(func(ts *taskScheduler) error {
		ts.executorsCount = c
		return nil
	})
}

func createTerminal() (terminalapi.Terminal, error) {
	if runtime.GOOS == "windows" {
		return tcell.New()
	}

	return termbox.New(termbox.ColorMode(terminalapi.ColorMode216))
}

func WithConsoleUserInterface(opts ...cui.Option) Option {

	return option(func(ts *taskScheduler) error {
		t, err := createTerminal()
		if err != nil {
			return err
		}

		ts.c, err = cui.NewCui(t, opts...)
		if err != nil {
			return err
		}

		return nil
	})
}



func WithTask(name string, f func (context.Context) error) Option {
	return option(func(ts *taskScheduler) error {
		if name == "" {
			return errors.New("task name must not be empty")
		}

		t := task.NewBaseTask(f, name)
		ts.exec.AddTask(t)
		return nil
	})
}

func WithTimeLimitedTask(name string, timeout time.Duration, f func (context.Context) error) Option {
	return option(func(ts *taskScheduler) error {
		if name == "" {
			return errors.New("task name must not be empty")
		}
		if timeout <= 0 {
			return errors.New("time-limited task timeout must be > 0")
		}

		t := task.NewTimeLimitedTask(f, name, timeout)
		ts.exec.AddTask(t)
		return nil
	})
}
