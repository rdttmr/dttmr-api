package domain

import (
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownEquipment lists every valid equipment with its database name. It must
// match the CHECK constraint on exercises.equipment.
var knownEquipment = []struct {
	equipment Equipment
	name      string
}{
	{EquipmentFloor, "floor"},
	{EquipmentRings, "rings"},
	{EquipmentPullUpBar, "pull_up_bar"},
	{EquipmentParallelBars, "parallel_bars"},
	{EquipmentLowBar, "low_bar"},
	{EquipmentParallettes, "parallettes"},
	{EquipmentResistanceBand, "resistance_band"},
}

func TestEquipment_KnownEquipmentIsComplete(t *testing.T) {
	// EquipmentUnknown has no name, every other value must be listed above.
	assert.Len(t, knownEquipment, len(equipmentNames)-1, "an Equipment was added without updating knownEquipment")
}

func TestEquipment_String(t *testing.T) {
	for _, tt := range knownEquipment {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.name, tt.equipment.String())
		})
	}

	outOfRange := []Equipment{EquipmentUnknown, -1, Equipment(len(equipmentNames)), Equipment(len(equipmentNames) + 1)}
	for _, e := range outOfRange {
		t.Run(fmt.Sprintf("no name for %d", int(e)), func(t *testing.T) {
			var name string
			require.NotPanics(t, func() { name = e.String() })
			assert.Empty(t, name)
		})
	}
}

func TestParseEquipment(t *testing.T) {
	for _, tt := range knownEquipment {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEquipment(tt.name)

			require.NoError(t, err)
			assert.Equal(t, tt.equipment, got)
		})
	}

	for _, input := range []string{"", "trampoline", "Rings", " rings", "pull-up-bar"} {
		t.Run(fmt.Sprintf("rejects %q", input), func(t *testing.T) {
			got, err := ParseEquipment(input)

			assert.Error(t, err)
			assert.Equal(t, EquipmentUnknown, got)
		})
	}
}

func TestEquipmentSet_Scan(t *testing.T) {
	accepted := []struct {
		name string
		src  any
		want EquipmentSet
	}{
		{name: "several items", src: "{floor,rings,parallettes}", want: EquipmentSet{EquipmentFloor, EquipmentRings, EquipmentParallettes}},
		{name: "one item", src: "{pull_up_bar}", want: EquipmentSet{EquipmentPullUpBar}},
		{name: "bytes", src: []byte("{low_bar,resistance_band}"), want: EquipmentSet{EquipmentLowBar, EquipmentResistanceBand}},
		{name: "empty array", src: "{}", want: EquipmentSet{}},
		{name: "surrounding whitespace", src: " {floor} ", want: EquipmentSet{EquipmentFloor}},
		{name: "NULL", src: nil, want: nil},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			s := EquipmentSet{EquipmentRings}

			require.NoError(t, s.Scan(tt.src))
			assert.Equal(t, tt.want, s)
		})
	}

	rejected := []struct {
		name string
		src  any
	}{
		{name: "unknown item", src: "{floor,trampoline}"},
		{name: "empty item", src: "{floor,}"},
		{name: "missing braces", src: "floor,rings"},
		{name: "missing closing brace", src: "{floor"},
		{name: "empty string", src: ""},
		{name: "unsupported type", src: 42},
	}
	for _, tt := range rejected {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			s := EquipmentSet{EquipmentRings}

			assert.Error(t, s.Scan(tt.src))
			assert.Equal(t, EquipmentSet{EquipmentRings}, s, "set must stay unchanged on error")
		})
	}
}

func TestEquipmentSet_Value(t *testing.T) {
	t.Run("writes a Postgres text array that scans back", func(t *testing.T) {
		set := EquipmentSet{EquipmentFloor, EquipmentRings, EquipmentResistanceBand}

		v, err := set.Value()

		require.NoError(t, err)
		assert.Equal(t, "{floor,rings,resistance_band}", v)

		var scanned EquipmentSet
		require.NoError(t, scanned.Scan(v))
		assert.Equal(t, set, scanned)
	})

	t.Run("empty and nil sets are empty arrays", func(t *testing.T) {
		for _, set := range []EquipmentSet{{}, nil} {
			v, err := set.Value()

			require.NoError(t, err)
			assert.Equal(t, "{}", v)
		}
	})

	for _, e := range []Equipment{EquipmentUnknown, -1, Equipment(len(equipmentNames))} {
		t.Run(fmt.Sprintf("rejects %d, which has no name", int(e)), func(t *testing.T) {
			var err error
			require.NotPanics(t, func() { _, err = EquipmentSet{EquipmentFloor, e}.Value() })
			assert.Error(t, err)
		})
	}
}

func TestEquipment_JSON(t *testing.T) {
	for _, tt := range knownEquipment {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.equipment)

			require.NoError(t, err)
			assert.JSONEq(t, `"`+tt.name+`"`, string(b))

			var back Equipment
			require.NoError(t, json.Unmarshal(b, &back))
			assert.Equal(t, tt.equipment, back)
		})
	}

	for _, e := range []Equipment{EquipmentUnknown, -1, Equipment(len(equipmentNames))} {
		t.Run(fmt.Sprintf("refuses to marshal %d, which has no name", int(e)), func(t *testing.T) {
			var err error
			require.NotPanics(t, func() { _, err = json.Marshal(e) })
			assert.Error(t, err)
		})
	}

	rejected := []string{`"trampoline"`, `""`, `1`}
	for _, input := range rejected {
		t.Run("refuses to unmarshal "+input, func(t *testing.T) {
			e := knownEquipment[0].equipment

			err := json.Unmarshal([]byte(input), &e)

			assert.Error(t, err)
			assert.Equal(t, knownEquipment[0].equipment, e, "value must stay unchanged")
		})
	}

	// json/v2 sets the zero value for null without calling UnmarshalText, so
	// null silently becomes EquipmentUnknown. Request payloads have to check for it.
	t.Run("null becomes EquipmentUnknown", func(t *testing.T) {
		e := EquipmentFloor

		require.NoError(t, json.Unmarshal([]byte(`null`), &e))
		assert.Equal(t, EquipmentUnknown, e)
	})
}
