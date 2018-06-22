package appoptics

import (
	"bytes"
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	metrics "github.com/go-kit/kit/metrics2"
	internalhistogram "github.com/go-kit/kit/metrics2/internal/histogram"
	"github.com/go-kit/kit/metrics2/internal/keyval"
)

const (
	defaultBatchSize = 300
	defaultPeriod    = time.Minute
)

type Provider struct {
	mtx sync.Mutex

	labelValues []string
	points      map[string]*point
	period      time.Duration

	retryMax   int
	retryDelay time.Duration

	batchSize int
	logError  func(err error)
}

type OptionFunc func(*Provider)

func WithLabelValues(labelValues ...string) OptionFunc {
	return func(p *Provider) {
		p.labelValues = labelValues
	}
}

func WithPeriod(period time.Duration) OptionFunc {
	return func(p *Provider) {
		p.period = period
	}
}

func WithRetry(max int, delay time.Duration) OptionFunc {
	return func(p *Provider) {
		p.retryMax = max
		p.retryDelay = delay
	}
}

func NewProvider(opts ...OptionFunc) *Provider {
	p := &Provider{
		points:    map[string]*point{},
		logError:  func(err error) {},
		period:    defaultPeriod,
		batchSize: defaultBatchSize,
	}

	for _, o := range opts {
		o(p)
	}

	return p
}

func (p *Provider) SendLoop(ctx context.Context, c <-chan time.Time, url *url.URL) {
	url, user, pass := extractCredentials(url)

	for {
		select {
		case <-ctx.Done():
			return
		case <-c:
			if err := p.write(ctx, url, user, pass); err != nil {
				p.logError(err)
			}
		}
	}
}

func (p *Provider) write(ctx context.Context, url *url.URL, user, pass string) error {
	requests, err := p.batchRequests(url, user, pass)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g := &errgroup{cancel: cancel}
	for _, req := range requests {
		req := req
		g.Go(func() error {
			return p.retry(func() error { return p.post(req.WithContext(ctx)) })
		})
	}

	return g.Wait()
}

func (p *Provider) retry(work func() error) error {
	retries := 0
	for {
		err := work()
		if retries >= p.retryMax {
			return err
		}
		time.Sleep(p.retryDelay)
		retries++
	}
}

func (p *Provider) post(req *http.Request) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return errUnexpectedCode{code: resp.StatusCode}
	}
	return nil
}

func (p *Provider) batchRequests(u *url.URL, user, pass string) ([]*http.Request, error) {
	measurements := p.sample()
	if len(measurements) == 0 {
		return nil, nil
	}

	u = u.ResolveReference(&url.URL{Path: "/v1/measurements"})
	nextEnd := func(e int) int {
		e += p.batchSize
		if l := len(measurements); e > l {
			return l
		}
		return e
	}

	requests := make([]*http.Request, 0, len(measurements)/p.batchSize+1)
	for batch, e := 0, nextEnd(0); batch < len(measurements); batch, e = e, nextEnd(e) {
		r := struct {
			Measurements []measurement `json:"measurements"`
		}{
			Measurements: measurements[batch:e],
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(r); err != nil {
			return nil, err
		}

		req, err := http.NewRequest(http.MethodPost, u.String(), &buf)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(user, pass)
		req.Header.Set("Content-Type", "application/json")
		requests = append(requests, req)
	}

	return requests, nil
}

func (p *Provider) sample() []measurement {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	ts := time.Now().Truncate(p.period)
	period := int(p.period.Seconds())

	ms := make([]measurement, 0, len(p.points))
	for _, point := range p.points {
		switch {
		case point.float != nil:
			v := point.float.Value()
			if point.reset {
				point.float.Set(0)
			}

			ms = append(ms, measurement{
				Name:       point.name,
				Time:       ts.Unix(),
				Period:     period,
				Attributes: attributes{Aggregate: true},
				Tags:       point.keyvals,
				Value:      v,
			})

		case point.histogram != nil:
			for _, pair := range []struct {
				suffix   string
				quantile float64
			}{
				{".perc50", 0.50},
				{".perc90", 0.90},
				{".perc95", 0.95},
				{".perc99", 0.99},
			} {
				ms = append(ms, measurement{
					Name:       point.name + pair.suffix,
					Time:       ts.Unix(),
					Period:     period,
					Attributes: attributes{Aggregate: true},
					Tags:       point.keyvals,
					Value:      point.histogram.Quantile(pair.quantile),
				})

			}
		}
	}

	return ms
}

type measurement struct {
	Name   string `json:"name"`
	Time   int64  `json:"time"`
	Period int    `json:"period"`

	Attributes attributes        `json:"attributes,omitempty"`
	Tags       map[string]string `json:"tags"`

	Value float64 `json:"value"`
}

type attributes struct {
	Aggregate bool `json:"aggregate"`
}

type errUnexpectedCode struct {
	code int
}

func (e errUnexpectedCode) Error() string {
	return fmt.Sprintf("Expected 2xx, got %d", e.code)
}

type point struct {
	name    string
	keyvals map[string]string
	reset   bool

	float     *expvar.Float
	histogram *internalhistogram.Histogram
}

func (p *point) Add(delta float64) {
	p.float.Add(delta)
}

func (p *point) Set(v float64) {
	p.float.Set(v)
}

func (p *point) Observe(v float64) {
	p.histogram.Observe(v)
}

func (p *Provider) NewCounter(id metrics.Identifier) metrics.Counter {
	return &counter{
		parent:  p,
		name:    id.Name,
		keyvals: p.keyvals(id.Labels),
		labels:  p.labels(id.Labels),
	}
}

func (p *Provider) NewGauge(id metrics.Identifier) metrics.Gauge {
	return &gauge{
		parent:  p,
		name:    id.Name,
		keyvals: p.keyvals(id.Labels),
		labels:  p.labels(id.Labels),
	}
}

func (p *Provider) NewHistogram(id metrics.Identifier) metrics.Histogram {
	return &histogram{
		parent:  p,
		name:    id.Name,
		keyvals: p.keyvals(id.Labels),
		labels:  p.labels(id.Labels),
	}
}

func (p *Provider) point(key, name string, reset bool, keyvals map[string]string) *point {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if _, ok := p.points[key]; !ok {
		p.points[key] = &point{name: name, reset: reset, keyvals: keyvals, float: new(expvar.Float)}
	}
	return p.points[key]
}

func (p *Provider) observe(key, name string, keyvals map[string]string, value float64) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if _, ok := p.points[key]; !ok {
		p.points[key] = &point{
			name:      name,
			keyvals:   keyvals,
			histogram: internalhistogram.New(),
		}
	}

	p.points[key].Observe(value)
}

// keyvals uses labels as a starting point and then applies all the default
// labelValues defined in the provider.
func (p *Provider) keyvals(labels []string) map[string]string {
	result := keyval.MakeWith(labels)
	for i := 0; i < len(p.labelValues); i += 2 {
		result[p.labelValues[i]] = p.labelValues[i+1]
	}
	return result
}

// labels generates the labels from all the keys defined in labelValues and
// concatenating that with labels.
func (p *Provider) labels(labels []string) []string {
	result := make([]string, 0, len(p.labelValues)/2)
	for i := 0; i < len(p.labelValues); i += 2 {
		result = append(result, p.labelValues[i])
	}
	return append(result, labels...)
}

type counter struct {
	parent  *Provider
	name    string
	keyvals map[string]string
	labels  []string
}

func (c *counter) With(keyvals ...string) metrics.Counter {
	return &counter{
		parent:  c.parent,
		name:    c.name,
		keyvals: keyval.Merge(c.keyvals, keyvals...),
		labels:  c.labels,
	}
}

func (c *counter) Add(delta float64) {
	key := key(c.name, c.labels, c.keyvals)
	c.parent.point(key, c.name, true, c.keyvals).Add(delta)
}

type gauge struct {
	parent  *Provider
	name    string
	labels  []string
	keyvals map[string]string
}

func (g *gauge) With(keyvals ...string) metrics.Gauge {
	return &gauge{
		parent:  g.parent,
		name:    g.name,
		keyvals: keyval.Merge(g.keyvals, keyvals...),
		labels:  g.labels,
	}
}

func (g *gauge) Set(value float64) {
	key := key(g.name, g.labels, g.keyvals)
	g.parent.point(key, g.name, false, g.keyvals).Set(value)
}

func (g *gauge) Add(delta float64) {
	key := key(g.name, g.labels, g.keyvals)
	g.parent.point(key, g.name, false, g.keyvals).Add(delta)
}

type histogram struct {
	parent  *Provider
	name    string
	labels  []string
	keyvals map[string]string
}

func (h *histogram) With(keyvals ...string) metrics.Histogram {
	return &histogram{
		parent:  h.parent,
		name:    h.name,
		keyvals: keyval.Merge(h.keyvals, keyvals...),
		labels:  h.labels,
	}
}

func (h *histogram) Observe(value float64) {
	key := key(h.name, h.labels, h.keyvals)
	h.parent.observe(key, h.name, h.keyvals, value)
}

func key(name string, labels []string, keyvals map[string]string) string {
	values := make([]string, len(labels))

	for idx, l := range labels {
		v, ok := keyvals[l]
		if !ok {
			v = metrics.UnknownValue
		}
		values[idx] = l + ":" + v
	}
	return name + "__" + strings.Join(values, "__")
}

func extractCredentials(url *url.URL) (*url.URL, string, string) {
	result := *url
	username := ""
	password := ""
	if url.User != nil {
		username = url.User.Username()
		password, _ = url.User.Password()
	}
	result.User = nil
	return &result, username, password
}
