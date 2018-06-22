package appoptics

import (
	"sync"
)

type errgroup struct {
	wg      sync.WaitGroup
	cancel  func()
	err     error
	errOnce sync.Once
}

func (g *errgroup) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

func (g *errgroup) Go(f func() error) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()

		if err := f(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	}()
}
