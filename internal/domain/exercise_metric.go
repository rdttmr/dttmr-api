package domain

import (
	"database/sql/driver"
	"fmt"
)

type Metric int

const (
	MetricUnknown Metric = iota
	MetricReps
	MetricSeconds
)

var metricNames = [...]string{
	MetricUnknown: "",
	MetricReps:    "reps",
	MetricSeconds: "seconds",
}

var metricValues = func() map[string]Metric {
	m := make(map[string]Metric, len(metricNames))
	for i, name := range metricNames {
		m[name] = Metric(i)
	}
	return m
}()

func (m Metric) String() string {
	if m < 0 || int(m) > len(metricNames) {
		return ""
	}
	return metricNames[m]
}

func ParseMetric(s string) (Metric, error) {
	if m, ok := metricValues[s]; ok {
		return m, nil
	}
	return MetricUnknown, fmt.Errorf("metric: unknown value %q", s)
}

//func (m Metric) MarshalJSON() ([]byte, error) {
//	s := m.String()
//	if s == "" {
//		return nil, fmt.Errorf("metric: cannot marshal value %d", int(m))
//	}
//	return json.Marshal(s)
//}
//
//func (m *Metric) UnmarshalJSON(data []byte) error {
//	var s string
//	if err := json.Unmarshal(data, &s); err != nil {
//		return err
//	}
//
//	v, err := ParseMetric(s)
//	if err != nil {
//		return err
//	}
//
//	*m = v
//	return nil
//}

func (m *Metric) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		return fmt.Errorf("metric: unexpected NULL")
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("metric: cannot scan %T", src)
	}

	parsed, err := ParseMetric(s)
	if err != nil {
		return err
	}

	*m = parsed
	return nil
}

func (m Metric) Value() (driver.Value, error) {
	s := m.String()
	if s == "" {
		return nil, fmt.Errorf("metric: cannot store value %d", int(m))
	}
	return s, nil
}
