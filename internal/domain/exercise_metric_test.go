package domain

import (
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownMetrics lists every valid metric with its database name. It must match
// the CHECK constraint on exercises.metric.
var knownMetrics = []struct {
	metric Metric
	name   string
}{
	{MetricReps, "reps"},
	{MetricSeconds, "seconds"},
}

func TestMetric_KnownMetricsAreComplete(t *testing.T) {
	// MetricUnknown has no name, every other value must be listed above.
	assert.Len(t, knownMetrics, len(metricNames)-1, "a Metric was added without updating knownMetrics")
}

func TestMetric_String(t *testing.T) {
	for _, tt := range knownMetrics {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.name, tt.metric.String())
		})
	}

	outOfRange := []Metric{MetricUnknown, -1, Metric(len(metricNames)), Metric(len(metricNames) + 1)}
	for _, m := range outOfRange {
		t.Run(fmt.Sprintf("no name for %d", int(m)), func(t *testing.T) {
			var name string
			require.NotPanics(t, func() { name = m.String() })
			assert.Empty(t, name)
		})
	}
}

func TestParseMetric(t *testing.T) {
	for _, tt := range knownMetrics {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMetric(tt.name)

			require.NoError(t, err)
			assert.Equal(t, tt.metric, got)
		})
	}

	for _, input := range []string{"", "meters", "Reps", " reps"} {
		t.Run(fmt.Sprintf("rejects %q", input), func(t *testing.T) {
			got, err := ParseMetric(input)

			assert.Error(t, err)
			assert.Equal(t, MetricUnknown, got)
		})
	}
}

func TestMetric_Scan(t *testing.T) {
	t.Run("string and bytes", func(t *testing.T) {
		for _, src := range []any{"seconds", []byte("seconds")} {
			var m Metric
			require.NoError(t, m.Scan(src))
			assert.Equal(t, MetricSeconds, m)
		}
	})

	rejected := []struct {
		name string
		src  any
	}{
		{name: "NULL", src: nil},
		{name: "unknown name", src: "meters"},
		{name: "empty string", src: ""},
		{name: "unsupported type", src: 1},
	}
	for _, tt := range rejected {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			m := MetricReps

			assert.Error(t, m.Scan(tt.src))
			assert.Equal(t, MetricReps, m, "value must stay unchanged on error")
		})
	}
}

func TestMetric_Value(t *testing.T) {
	for _, tt := range knownMetrics {
		t.Run(tt.name, func(t *testing.T) {
			v, err := tt.metric.Value()

			require.NoError(t, err)
			assert.Equal(t, tt.name, v)

			var scanned Metric
			require.NoError(t, scanned.Scan(v))
			assert.Equal(t, tt.metric, scanned)
		})
	}

	for _, m := range []Metric{MetricUnknown, -1, Metric(len(metricNames))} {
		t.Run(fmt.Sprintf("rejects %d, which has no name", int(m)), func(t *testing.T) {
			var err error
			require.NotPanics(t, func() { _, err = m.Value() })
			assert.Error(t, err)
		})
	}
}

func TestMetric_JSON(t *testing.T) {
	for _, tt := range knownMetrics {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.metric)

			require.NoError(t, err)
			assert.JSONEq(t, `"`+tt.name+`"`, string(b))

			var back Metric
			require.NoError(t, json.Unmarshal(b, &back))
			assert.Equal(t, tt.metric, back)
		})
	}

	for _, m := range []Metric{MetricUnknown, -1, Metric(len(metricNames))} {
		t.Run(fmt.Sprintf("refuses to marshal %d, which has no name", int(m)), func(t *testing.T) {
			var err error
			require.NotPanics(t, func() { _, err = json.Marshal(m) })
			assert.Error(t, err)
		})
	}

	rejected := []string{`"meters"`, `""`, `1`}
	for _, input := range rejected {
		t.Run("refuses to unmarshal "+input, func(t *testing.T) {
			m := knownMetrics[0].metric

			err := json.Unmarshal([]byte(input), &m)

			assert.Error(t, err)
			assert.Equal(t, knownMetrics[0].metric, m, "value must stay unchanged")
		})
	}

	// json/v2 sets the zero value for null without calling UnmarshalText, so
	// null silently becomes MetricUnknown. Request payloads have to check for it.
	t.Run("null becomes MetricUnknown", func(t *testing.T) {
		m := MetricReps

		require.NoError(t, json.Unmarshal([]byte(`null`), &m))
		assert.Equal(t, MetricUnknown, m)
	})
}
