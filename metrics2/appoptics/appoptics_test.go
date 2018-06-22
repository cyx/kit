package appoptics

import (
	"net/url"
	"testing"

	metrics "github.com/go-kit/kit/metrics2"
)

func TestCounter(t *testing.T) {
	tests := []struct {
		name string
		p    *Provider
		fn   func(p *Provider)

		wantKey string
		wantVal float64
	}{
		{
			name: "default provider, no labels, no keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewCounter(metrics.Identifier{Name: "test.counter"}).Add(1)
			},
			wantKey: key("test.counter", nil, nil),
			wantVal: 1,
		},

		{
			name: "default provider, one label, no keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewCounter(metrics.Identifier{
					Name:   "test.counter",
					Labels: []string{"region"},
				}).Add(1)
			},
			wantKey: key("test.counter", []string{"region"}, nil),
			wantVal: 1,
		},
		{
			name: "default provider, one label, specified keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewCounter(metrics.Identifier{
					Name:   "test.counter",
					Labels: []string{"region"},
				}).With("region", "us").Add(1)
			},
			wantKey: key("test.counter", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, no specified keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewCounter(metrics.Identifier{
					Name: "test.counter",
				}).Add(1)
			},
			wantKey: key("test.counter", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, no specified keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewCounter(metrics.Identifier{
					Name: "test.counter",
				}).Add(1)
			},
			wantKey: key("test.counter", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, overridden keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewCounter(metrics.Identifier{
					Name: "test.counter",
				}).With("region", "eu").Add(1)
			},
			wantKey: key("test.counter", []string{"region"}, map[string]string{"region": "eu"}),
			wantVal: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fn(test.p)
			point := test.p.points[test.wantKey]

			if point == nil {
				t.Fatalf("wanted key %q to be set", test.wantKey)
			}

			if point.reset != true {
				t.Fatal("wanted reset to be true for counter, got false")
			}

			if gotVal := point.float.Value(); test.wantVal != gotVal {
				t.Fatalf("wanted val %f, got %f (key=%q)", test.wantVal, gotVal, test.wantKey)
			}
		})
	}
}

func TestGauge(t *testing.T) {
	tests := []struct {
		name string
		p    *Provider
		fn   func(p *Provider)

		wantKey string
		wantVal float64
	}{
		{
			name: "default provider, no labels, no keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewGauge(metrics.Identifier{Name: "test.gauge"}).Add(1)
			},
			wantKey: key("test.gauge", nil, nil),
			wantVal: 1,
		},

		{
			name: "default provider, one label, no keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewGauge(metrics.Identifier{
					Name:   "test.gauge",
					Labels: []string{"region"},
				}).Add(1)
			},
			wantKey: key("test.gauge", []string{"region"}, nil),
			wantVal: 1,
		},
		{
			name: "default provider, one label, specified keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewGauge(metrics.Identifier{
					Name:   "test.gauge",
					Labels: []string{"region"},
				}).With("region", "us").Add(1)
			},
			wantKey: key("test.gauge", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, no specified keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewGauge(metrics.Identifier{
					Name: "test.gauge",
				}).Add(1)
			},
			wantKey: key("test.gauge", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, no specified keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewGauge(metrics.Identifier{
					Name: "test.gauge",
				}).Add(1)
			},
			wantKey: key("test.gauge", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, overridden keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewGauge(metrics.Identifier{
					Name: "test.gauge",
				}).With("region", "eu").Add(1)
			},
			wantKey: key("test.gauge", []string{"region"}, map[string]string{"region": "eu"}),
			wantVal: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fn(test.p)
			point := test.p.points[test.wantKey]

			if point == nil {
				t.Fatalf("wanted key %q to be set", test.wantKey)
			}

			if point.reset != false {
				t.Fatal("wanted reset to be false for gauge, got true")
			}

			if gotVal := point.float.Value(); test.wantVal != gotVal {
				t.Fatalf("wanted val %f, got %f (key=%q)", test.wantVal, gotVal, test.wantKey)
			}
		})
	}
}

func TestHistogram(t *testing.T) {
	tests := []struct {
		name string
		p    *Provider
		fn   func(p *Provider)

		wantKey string
		wantVal float64
	}{
		{
			name: "default provider, no labels, no keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewHistogram(metrics.Identifier{Name: "test.histogram"}).Observe(1)
			},
			wantKey: key("test.histogram", nil, nil),
			wantVal: 1,
		},

		{
			name: "default provider, one label, no keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewHistogram(metrics.Identifier{
					Name:   "test.histogram",
					Labels: []string{"region"},
				}).Observe(1)
			},
			wantKey: key("test.histogram", []string{"region"}, nil),
			wantVal: 1,
		},
		{
			name: "default provider, one label, specified keyvals",
			p:    NewProvider(),
			fn: func(p *Provider) {
				p.NewHistogram(metrics.Identifier{
					Name:   "test.histogram",
					Labels: []string{"region"},
				}).With("region", "us").Observe(1)
			},
			wantKey: key("test.histogram", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, no specified keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewHistogram(metrics.Identifier{
					Name: "test.histogram",
				}).Observe(1)
			},
			wantKey: key("test.histogram", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, no specified keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewHistogram(metrics.Identifier{
					Name: "test.histogram",
				}).Observe(1)
			},
			wantKey: key("test.histogram", []string{"region"}, map[string]string{"region": "us"}),
			wantVal: 1,
		},
		{
			name: "provider with default keyvals, no label, overridden keyvals",
			p:    NewProvider(WithLabelValues("region", "us")),
			fn: func(p *Provider) {
				p.NewHistogram(metrics.Identifier{
					Name: "test.histogram",
				}).With("region", "eu").Observe(1)
			},
			wantKey: key("test.histogram", []string{"region"}, map[string]string{"region": "eu"}),
			wantVal: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fn(test.p)
			point := test.p.points[test.wantKey]

			if point == nil {
				t.Fatalf("wanted key %q to be set", test.wantKey)
			}

			if gotVal := point.histogram.Quantile(0.99); test.wantVal != gotVal {
				t.Fatalf("wanted val %f, got %f (key=%q)", test.wantVal, gotVal, test.wantKey)
			}
		})
	}
}

func TestExtractCredentials(t *testing.T) {
	u, _ := url.Parse("https://foo:bar@example.com")
	cleanURL, user, pass := extractCredentials(u)

	if "foo" != user {
		t.Fatalf("want user: %q, got %q", "foo", user)
	}

	if "bar" != pass {
		t.Fatalf("want pass: %q, got %q", "bar", pass)
	}

	if want, got := "https://foo:bar@example.com", u.String(); want != got {
		t.Fatalf("want original url: %q, got %q", want, got)
	}

	if want, got := "https://example.com", cleanURL.String(); want != got {
		t.Fatalf("want clean url: %q, got %q", want, got)
	}
}
