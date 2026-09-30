package repository_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.dittmar.dev/robin/dttmr-api/internal/repository"
)

func TestNewStore(t *testing.T) {
	store := repository.NewStore(nil)

	require.NotNil(t, store)
	require.NotNil(t, store.Transactor)
}

// Walks all fields of Store via reflection, so a repo added to the struct but
// forgotten in NewStore fails here without touching this test.
func TestNewStore_AllReposShareTheTransactor(t *testing.T) {
	store := repository.NewStore(nil)

	v := reflect.ValueOf(store).Elem()
	typ := v.Type()

	for i := range v.NumField() {
		field := typ.Field(i)
		if field.Name == "Transactor" {
			continue
		}

		t.Run(field.Name, func(t *testing.T) {
			repo := v.Field(i)
			require.Equal(t, reflect.Pointer, repo.Kind(), "Store.%s is not a pointer", field.Name)
			require.False(t, repo.IsNil(), "Store.%s is not initialized by NewStore", field.Name)

			tr := repo.Elem().FieldByName("Transactor")
			require.True(t, tr.IsValid(), "Store.%s has no Transactor", field.Name)
			assert.Same(t, store.Transactor, tr.Interface(), "Store.%s uses a different Transactor", field.Name)
		})
	}
}
