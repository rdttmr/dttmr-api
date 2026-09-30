package database_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"git.dittmar.dev/robin/dttmr-api/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pgx driver parses the URL lazily, so a malformed URL only surfaces
// when New pings the database.
func TestNew_MalformedURL(t *testing.T) {
	db, err := database.New(context.Background(), "postgres://user:pass@localhost:notaport/db")

	assert.Nil(t, db)
	assert.ErrorContains(t, err, "invalid port")
}

func TestNew_UnreachableServer(t *testing.T) {
	// Reserve a free port and close it again, so nothing listens there.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.New(ctx, "postgres://user:pass@"+addr+"/db?sslmode=disable&connect_timeout=5")

	assert.Nil(t, db)
	assert.ErrorContains(t, err, "database unreachable")
}

func TestNew_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	db, err := database.New(ctx, "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable")

	assert.Nil(t, db)
	assert.ErrorIs(t, err, context.Canceled)
	assert.ErrorContains(t, err, "database unreachable")
}

// Needs a real database; runs in the migrations CI job, skipped elsewhere.
func TestNew_ConfiguresThePool(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	assert.Equal(t, 10, db.Stats().MaxOpenConnections)

	var one int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT 1").Scan(&one))
	assert.Equal(t, 1, one)
}
