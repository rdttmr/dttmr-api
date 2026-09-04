package domain

import (
	"database/sql/driver"
	"fmt"
)

type Load int

const (
	LoadUnknown Load = iota
	LoadBodyweight
	LoadExternal
)

var loadNames = [...]string{
	LoadUnknown:    "",
	LoadBodyweight: "bodyweight",
	LoadExternal:   "external",
}

var loadValues = func() map[string]Load {
	m := make(map[string]Load, len(loadNames))
	for i, name := range loadNames {
		m[name] = Load(i)
	}
	return m
}()

func (l Load) String() string {
	if l < 0 || int(l) > len(loadNames) {
		return ""
	}
	return loadNames[l]
}

func ParseLoad(s string) (Load, error) {
	if l, ok := loadValues[s]; ok {
		return l, nil
	}
	return LoadUnknown, fmt.Errorf("load: unknown value %q", s)
}

//func (l Load) MarshalJSON() ([]byte, error) {
//	s := l.String()
//	if s == "" {
//		return nil, fmt.Errorf("load: cannot marshal value %d", int(l))
//	}
//	return json.Marshal(s)
//}
//
//func (l *Load) UnmarshalJSON(data []byte) error {
//	var s string
//	if err := json.Unmarshal(data, &s); err != nil {
//		return err
//	}
//
//	v, err := ParseLoad(s)
//	if err != nil {
//		return err
//	}
//
//	*l = v
//	return nil
//}

func (l *Load) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		return fmt.Errorf("load: unexpected NULL")
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("load: cannot scan %T", src)
	}

	parsed, err := ParseLoad(s)
	if err != nil {
		return err
	}

	*l = parsed
	return nil
}

func (l Load) Value() (driver.Value, error) {
	s := l.String()
	if s == "" {
		return nil, fmt.Errorf("load: cannot store value %d", int(l))
	}
	return s, nil
}
