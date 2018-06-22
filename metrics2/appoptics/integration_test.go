// +build integration

// http://peter.bourgon.org/go-in-production/#testing-and-validation
package appoptics

import (
	"context"
	"flag"
	"log"
	"net/url"
	"testing"
	"time"

	metrics "github.com/go-kit/kit/metrics2"
)

var appOpticsURL = flag.String("appoptics-url", "", "")

func TestPostingMetrics(t *testing.T) {
	p := NewProvider()
	c := p.NewCounter(metrics.Identifier{Name: "appoptics.test.counter", Labels: []string{"region", "system"}})
	g := p.NewGauge(metrics.Identifier{Name: "appoptics.test.gauge", Labels: []string{"region", "system"}})
	h := p.NewHistogram(metrics.Identifier{Name: "appoptics.test.histogram", Labels: []string{"region", "system"}})

	c.With("region", "us").With("system", "test").Add(100)
	g.With("region", "us").With("system", "test").Set(1000)
	h.With("region", "us").With("system", "test").Observe(10)
	h.With("region", "us").With("system", "test").Observe(50)
	h.With("region", "us").With("system", "test").Observe(1000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	u, err := url.Parse(*appOpticsURL)
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan time.Time, 1)
	ch <- time.Now()

	log.Printf("SendLoop ensuing, url = %s", u)
	go p.SendLoop(ctx, ch, u)
	time.Sleep(2 * time.Second)
	cancel()
}
