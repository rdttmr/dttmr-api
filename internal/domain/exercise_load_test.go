package domain

import (
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownLoads lists every valid load with its database name. It must match the
// CHECK constraint on exercises.load.
var knownLoads = []struct {
	load Load
	name string
}{
	{LoadBodyweight, "bodyweight"},
	{LoadExternal, "external"},
}

func TestLoad_KnownLoadsAreComplete(t *testing.T) {
	// LoadUnknown has no name, every other value must be listed above.
	assert.Len(t, knownLoads, len(loadNames)-1, "a Load was added without updating knownLoads")
}

func TestLoad_String(t *testing.T) {
	for _, tt := range knownLoads {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.name, tt.load.String())
		})
	}

	outOfRange := []Load{LoadUnknown, -1, Load(len(loadNames)), Load(len(loadNames) + 1)}
	for _, l := range outOfRange {
		t.Run(fmt.Sprintf("no name for %d", int(l)), func(t *testing.T) {
			var name string
			require.NotPanics(t, func() { name = l.String() })
			assert.Empty(t, name)
		})
	}
}

func TestParseLoad(t *testing.T) {
	for _, tt := range knownLoads {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLoad(tt.name)

			require.NoError(t, err)
			assert.Equal(t, tt.load, got)
		})
	}

	for _, input := range []string{"", "absolute", "Bodyweight", " bodyweight"} {
		t.Run(fmt.Sprintf("rejects %q", input), func(t *testing.T) {
			got, err := ParseLoad(input)

			assert.Error(t, err)
			assert.Equal(t, LoadUnknown, got)
		})
	}
}

func TestLoad_Scan(t *testing.T) {
	t.Run("string and bytes", func(t *testing.T) {
		for _, src := range []any{"external", []byte("external")} {
			var l Load
			require.NoError(t, l.Scan(src))
			assert.Equal(t, LoadExternal, l)
		}
	})

	rejected := []struct {
		name string
		src  any
	}{
		{name: "NULL", src: nil},
		{name: "unknown name", src: "absolute"},
		{name: "empty string", src: ""},
		{name: "unsupported type", src: 1},
	}
	for _, tt := range rejected {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			l := LoadBodyweight

			assert.Error(t, l.Scan(tt.src))
			assert.Equal(t, LoadBodyweight, l, "value must stay unchanged on error")
		})
	}
}

func TestLoad_Value(t *testing.T) {
	for _, tt := range knownLoads {
		t.Run(tt.name, func(t *testing.T) {
			v, err := tt.load.Value()

			require.NoError(t, err)
			assert.Equal(t, tt.name, v)

			var scanned Load
			require.NoError(t, scanned.Scan(v))
			assert.Equal(t, tt.load, scanned)
		})
	}

	for _, l := range []Load{LoadUnknown, -1, Load(len(loadNames))} {
		t.Run(fmt.Sprintf("rejects %d, which has no name", int(l)), func(t *testing.T) {
			var err error
			require.NotPanics(t, func() { _, err = l.Value() })
			assert.Error(t, err)
		})
	}
}

func TestLoad_JSON(t *testing.T) {
	for _, tt := range knownLoads {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.load)

			require.NoError(t, err)
			assert.JSONEq(t, `"`+tt.name+`"`, string(b))

			var back Load
			require.NoError(t, json.Unmarshal(b, &back))
			assert.Equal(t, tt.load, back)
		})
	}

	for _, l := range []Load{LoadUnknown, -1, Load(len(loadNames))} {
		t.Run(fmt.Sprintf("refuses to marshal %d, which has no name", int(l)), func(t *testing.T) {
			var err error
			require.NotPanics(t, func() { _, err = json.Marshal(l) })
			assert.Error(t, err)
		})
	}

	rejected := []string{`"absolute"`, `""`, `1`}
	for _, input := range rejected {
		t.Run("refuses to unmarshal "+input, func(t *testing.T) {
			l := knownLoads[0].load

			err := json.Unmarshal([]byte(input), &l)

			assert.Error(t, err)
			assert.Equal(t, knownLoads[0].load, l, "value must stay unchanged")
		})
	}

	// json/v2 sets the zero value for null without calling UnmarshalText, so
	// null silently becomes LoadUnknown. Request payloads have to check for it.
	t.Run("null becomes LoadUnknown", func(t *testing.T) {
		l := LoadBodyweight

		require.NoError(t, json.Unmarshal([]byte(`null`), &l))
		assert.Equal(t, LoadUnknown, l)
	})
}
