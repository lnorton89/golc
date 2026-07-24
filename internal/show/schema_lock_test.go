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
)

func TestOpenStoreWaitsOutContentionOnFirstStatement(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	// Establish the file first (creates it, stamps the GOLC application_id,
	// leaves it in WAL mode) so the contended openStore call below is
	// exercising a real, already-initialized GOLC store, not a fresh-file
	// creation race.
	if _, err := Load(root, path); err != nil {
		t.Fatalf("seeding initial store: %v", err)
	}

	resolved := resolvePath(root, path)

	blockerDB, err := sql.Open("sqlite", resolved)
	if err != nil {
		t.Fatalf("opening blocker connection: %v", err)
	}
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
	if _, err := blockerDB.Exec(`PRAGMA locking_mode = EXCLUSIVE`); err != nil {
		t.Fatalf("setting exclusive locking mode: %v", err)
	}
	if _, err := blockerDB.Exec(`CREATE TABLE IF NOT EXISTS lock_holder (id INTEGER)`); err != nil {
		t.Fatalf("forcing exclusive lock acquisition: %v", err)
	}

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
		if err := blockerDB.Close(); err != nil {
			t.Errorf("releasing exclusive lock via close: %v", err)
		}
	}()

	start := time.Now()
	_, loadErr := Load(root, path)
	elapsed := time.Since(start)
	wg.Wait()

	if loadErr != nil {
		t.Fatalf("Load contended with an exclusive lock held for %s should have waited it out via busy_timeout, got error instead: %v", holdDuration, loadErr)
	}
	if elapsed < holdDuration/2 {
		t.Fatalf("Load returned after only %s, which is suspiciously fast given a %s exclusive lock was held concurrently -- this test may not be exercising real contention", elapsed, holdDuration)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Load took %s, longer than the 5s busy_timeout ceiling -- contention was not resolved by the busy handler as expected", elapsed)
	}
	fmt.Printf("Load waited %s under contention (lock held %s) and succeeded\n", elapsed, holdDuration)
}
