package domain

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

type Equipment int

const (
	EquipmentUnknown Equipment = iota
	EquipmentFloor
	EquipmentRings
	EquipmentPullUpBar
	EquipmentParallelBars
	EquipmentLowBar
	EquipmentParallettes
	EquipmentResistanceBand
)

var equipmentNames = [...]string{
	EquipmentUnknown:        "",
	EquipmentFloor:          "floor",
	EquipmentRings:          "rings",
	EquipmentPullUpBar:      "pull_up_bar",
	EquipmentParallelBars:   "parallel_bars",
	EquipmentLowBar:         "low_bar",
	EquipmentParallettes:    "parallettes",
	EquipmentResistanceBand: "resistance_band",
}

var equipmentValues = func() map[string]Equipment {
	m := make(map[string]Equipment, len(equipmentNames))
	for i, name := range equipmentNames {
		m[name] = Equipment(i)
	}
	return m
}()

func (e Equipment) String() string {
	if e < 0 || int(e) > len(equipmentNames) {
		return ""
	}
	return equipmentNames[e]
}

func ParseEquipment(s string) (Equipment, error) {
	if e, ok := equipmentValues[s]; ok {
		return e, nil
	}
	return EquipmentUnknown, fmt.Errorf("equipment: unknown value %q", s)
}

//func (e Equipment) MarshalJSON() ([]byte, error) {
//	s := e.String()
//	if s == "" {
//		return nil, fmt.Errorf("equipment: cannot marshal value %d", int(e))
//	}
//	return json.Marshal(s)
//}
//
//func (e *Equipment) UnmarshalJSON(data []byte) error {
//	var s string
//	if err := json.Unmarshal(data, &s); err != nil {
//		return err
//	}
//
//	v, err := ParseEquipment(s)
//	if err != nil {
//		return err
//	}
//
//	*e = v
//	return nil
//}

type EquipmentSet []Equipment

func (s *EquipmentSet) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}

	var raw string
	switch v := src.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("equipment: cannot scan %T", src)
	}

	names, err := parseTextArray(raw)
	if err != nil {
		return err
	}
	out := make(EquipmentSet, len(names))
	for i, name := range names {
		e, err := ParseEquipment(name)
		if err != nil {
			return err
		}
		out[i] = e
	}
	*s = out
	return nil
}

func (s EquipmentSet) Value() (driver.Value, error) {
	names := make([]string, len(s))
	for i, e := range s {
		name := e.String()
		if name == "" {
			return nil, fmt.Errorf("equipment: cannot store value %d", int(e))
		}
		names[i] = name
	}

	return "{" + strings.Join(names, ",") + "}", nil
}

func parseTextArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, fmt.Errorf("equipment: malformed array %q", raw)
	}
	if inner := raw[1 : len(raw)-1]; inner != "" {
		return strings.Split(inner, ","), nil
	}
	return nil, nil
}
