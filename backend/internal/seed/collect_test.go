package seed

import (
	"errors"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type fakeBatchResults struct {
	affects  []int64
	execErr  error
	closeErr error
	calls    int
	closed   bool
}

func (f *fakeBatchResults) Exec() (pgconn.CommandTag, error) {
	f.calls++
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}

	affected := f.affects[min(f.calls-1, len(f.affects)-1)]

	return pgconn.NewCommandTag("INSERT 0 " + strconv.FormatInt(affected, 10)), nil
}

func (f *fakeBatchResults) Close() error {
	f.closed = true

	return f.closeErr
}

func TestCollectBatchSumsAffectedRows(t *testing.T) {
	results := &fakeBatchResults{affects: []int64{1, 2, 0}}

	inserted, err := collectBatch(results, 3)
	if err != nil {
		t.Fatalf("collectBatch() error = %v", err)
	}

	if inserted != 3 {
		t.Errorf("collectBatch() = %d, want 3", inserted)
	}

	if !results.closed {
		t.Error("collectBatch() did not close the results")
	}
}

func TestCollectBatchReportsExecFailure(t *testing.T) {
	results := &fakeBatchResults{affects: []int64{5}, execErr: errors.New("exec failed")}

	inserted, err := collectBatch(results, 4)
	if err == nil {
		t.Fatal("collectBatch() error = nil, want the exec failure")
	}

	if inserted != 0 {
		t.Errorf("collectBatch() = %d, want 0", inserted)
	}

	if !errors.Is(err, results.execErr) {
		t.Errorf("collectBatch() error = %v, want it to wrap the exec failure", err)
	}

	if results.calls != 1 {
		t.Errorf("Exec() calls = %d, want the loop to stop after the first failure", results.calls)
	}
}

func TestCollectBatchReportsCloseFailure(t *testing.T) {
	results := &fakeBatchResults{affects: []int64{7}, closeErr: errors.New("close failed")}

	inserted, err := collectBatch(results, 1)
	if err == nil {
		t.Fatal("collectBatch() error = nil, want the close failure")
	}

	if inserted != 0 {
		t.Errorf("collectBatch() = %d, want 0", inserted)
	}

	if !errors.Is(err, results.closeErr) {
		t.Errorf("collectBatch() error = %v, want it to wrap the close failure", err)
	}
}

func TestCollectBatchHandlesEmptyBatch(t *testing.T) {
	results := &fakeBatchResults{}

	inserted, err := collectBatch(results, 0)
	if err != nil {
		t.Fatalf("collectBatch() error = %v", err)
	}

	if inserted != 0 {
		t.Errorf("collectBatch() = %d, want 0", inserted)
	}

	if results.calls != 0 {
		t.Errorf("Exec() calls = %d, want none", results.calls)
	}
}
