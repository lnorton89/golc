// schema_lock_test.go regression-tests the busy_timeout pragma-ordering fix
// (openStore, schema.go): busy_timeout must be the FIRST pragma applied on a
// connection so it covers every later statement -- including the very first
// one -- rather than being set only after journal_mode=WAL has already had a
// chance to contend for a lock and fail immediately. This simulates that
// "another process holds the file locked at the moment a fresh connection's
// very first statement runs" race directly: a second, independently-opened
// *sql.DB grabs an exclusive, whole-file lock (PRAGMA locking_mode=EXCLUSIVE
// plus a write) for a short, known duration while openStore is invoked
// concurrently. With busy_timeout applied first, openStore's first pragma
// waits out the contention and succeeds; if busy_timeout were applied last
// (the pre-fix ordering), that first pragma would return SQLITE_BUSY
// immediately, before busy_timeout had ever been set on that connection.
package show

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenStoreWaitsOutContentionOnFirstStatement(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	// Establish the file first (creates it, stamps the GOLC application_id,
	// leaves it in WAL mode) so the contended openStore call below is
	// exercising a real, already-initialized GOLC store, not a fresh-file
	// creation race.
	_, err := Load(root, path)
	require.NoError(t, err, "seeding initial store")

	resolved := resolvePath(root, path)

	blockerDB, err := sql.Open("sqlite", resolved)
	require.NoError(t, err, "opening blocker connection")
	defer blockerDB.Close()
	blockerDB.SetMaxOpenConns(1)

	const holdDuration = 500 * time.Millisecond

	// PRAGMA locking_mode=EXCLUSIVE plus a write forces SQLite to take and
	// keep an OS-level exclusive lock on the whole file for the life of this
	// connection -- any other connection's very first statement against the
	// same file (including a read-only PRAGMA) is denied with SQLITE_BUSY
	// until this lock releases, exactly the class of "first statement on a
	// fresh connection contends with another process" race busy_timeout
	// ordering is meant to survive.
	_, err = blockerDB.Exec(`PRAGMA locking_mode = EXCLUSIVE`)
	require.NoError(t, err, "setting exclusive locking mode")
	_, err = blockerDB.Exec(`CREATE TABLE IF NOT EXISTS lock_holder (id INTEGER)`)
	require.NoError(t, err, "forcing exclusive lock acquisition")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(holdDuration)
		// PRAGMA locking_mode=NORMAL alone does not drop an already-taken
		// EXCLUSIVE lock (SQLite only releases it lazily, on this
		// connection's next access, if at all before close) -- closing the
		// connection outright is what deterministically releases it at a
		// known time.
		assert.NoError(t, blockerDB.Close(), "releasing exclusive lock via close")
	}()

	start := time.Now()
	_, loadErr := Load(root, path)
	elapsed := time.Since(start)
	wg.Wait()

	require.NoError(t, loadErr, "Load contended with an exclusive lock held for %s should have waited it out via busy_timeout", holdDuration)
	require.GreaterOrEqual(t, elapsed, holdDuration/2, "Load returned after only %s, which is suspiciously fast given a %s exclusive lock was held concurrently -- this test may not be exercising real contention", elapsed, holdDuration)
	require.LessOrEqual(t, elapsed, 5*time.Second, "Load took %s, longer than the 5s busy_timeout ceiling -- contention was not resolved by the busy handler as expected", elapsed)
	fmt.Printf("Load waited %s under contention (lock held %s) and succeeded\n", elapsed, holdDuration)
}
