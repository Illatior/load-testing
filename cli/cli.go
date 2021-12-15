package cli

import (
	"context"
	"github.com/illatior/nora/cli/screen"
	"github.com/illatior/nora/lib/metric"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"sync"
	"time"
)

const (
	SCREEN_ID = "scr"
)

type cui struct {
	isFullscreen bool

	t terminalapi.Terminal
	c *container.Container

	screenMu      sync.RWMutex
	currentScreen int
	screens       []screen.Screen

	updateInterval                 time.Duration
	changeDisplayableIntervalDelta time.Duration

	metricsMu sync.RWMutex
	metrics   metric.Metrics

	subs subsFunc

	done chan bool
}

func NewCui(t terminalapi.Terminal, opts ...Option) (ConsoleUserInterface, error) {
	c, err := container.New(
		t,
		container.ID(SCREEN_ID),
		container.Border(linestyle.Light),
		container.BorderTitle("Task-scheduler"),
	)
	if err != nil {
		return nil, err
	}

	mainScreen, err := screen.NewMainScreen()
	if err != nil {
		return nil, err
	}

	ui := &cui{
		isFullscreen: false,
		c:            c,
		t:            t,

		screenMu:      sync.RWMutex{},
		currentScreen: 0,
		screens:       []screen.Screen{mainScreen},

		updateInterval:                 100 * time.Millisecond,
		changeDisplayableIntervalDelta: 5 * time.Second,

		metrics: metric.NewMetrics(),

		subs: defaultSubs(),
		done: make(chan bool),
	}

	for _, opt := range opts {
		err = opt.apply(ui)
		if err != nil {
			return nil, err
		}
	}

	err = ui.changeMainScreen()
	if err != nil {
		return nil, err
	}

	return ui, nil
}

func (ui *cui) Run(ctx context.Context, metrics <-chan *metric.Result, dispatchDone <-chan bool) error {
	defer func() {
		ui.done <- true
	}()

	ctx, cancel := context.WithCancel(ctx)
	go ui.update(ctx, dispatchDone)
	go func() {
		for _, s := range ui.screens {
			go s.Run(ctx)
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-metrics:
				ui.metricsMu.Lock()
				ui.metrics.ConsumeResult(m)
				ui.metricsMu.Unlock()
			}
		}
	}()

	// TODO add ability to customize subs with option
	subs := ui.subs(ctx, cancel, ui)

	defer func() {
		ui.t.Close()
	}()
	return termdash.Run(ctx, ui.t, ui.c, termdash.KeyboardSubscriber(subs))
}

func (ui *cui) GetDoneChan() <-chan bool {
	return ui.done
}

func (ui *cui) update(ctx context.Context, dispatchDone <-chan bool) {
	t := time.NewTicker(ui.updateInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dispatchDone:
			return
		case <-t.C:
			currentScreen := ui.screens[ui.currentScreen]

			select {
			case <-ctx.Done():
				return
			case currentScreen.GetMetricsChan() <- ui.metrics:
				break
			}
		}
	}
}

func (ui *cui) ChangeFullscreenState() error {
	ui.isFullscreen = !ui.isFullscreen

	return ui.changeMainScreen()
}

func (ui *cui) NextScreen() error {
	ui.screenMu.Lock()
	defer ui.screenMu.Unlock()

	ui.currentScreen++
	if ui.currentScreen == len(ui.screens) {
		ui.currentScreen = 0
	}

	return ui.changeMainScreen()
}

func (ui *cui) PreviousScreen() error {
	ui.screenMu.Lock()
	defer ui.screenMu.Unlock()

	ui.currentScreen--
	if ui.currentScreen < 0 {
		ui.currentScreen = len(ui.screens) - 1
	}

	return ui.changeMainScreen()
}

func (ui *cui) changeMainScreen() error { // FIXME after exiting fullscreen mode main BorderTitle and BorderStyle continues to be as body's one
	currentScreen := ui.screens[ui.currentScreen]

	builder := grid.New()
	if ui.isFullscreen {
		addElem(currentScreen.GetBody(), builder)
	} else {
		addElem(currentScreen.GetHeader(), builder)
		addElem(currentScreen.GetBody(), builder)
		addElem(currentScreen.GetFooter(), builder)
	}

	opts, err := builder.Build()
	if err != nil {
		return err
	}

	return ui.c.Update(SCREEN_ID, opts...)
}

func (ui *cui) IsFullscreen() bool {
	return ui.isFullscreen
}
