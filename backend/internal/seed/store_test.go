package seed

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeTransaction struct {
	results   *fakeBatchResults
	commitErr error
	queued    int
	commits   int
}

func (f *fakeTransaction) SendBatch(_ context.Context, batch *pgx.Batch) batchResults {
	f.queued += batch.Len()

	return f.results
}

func (f *fakeTransaction) Commit(_ context.Context) error {
	f.commits++

	return f.commitErr
}

func TestStoreBatchReturnsInsertedCount(t *testing.T) {
	tx := &fakeTransaction{results: &fakeBatchResults{affects: []int64{1, 1}}}

	inserted, err := storeBatch(context.Background(), tx, "INSERT", twoRecords())
	if err != nil {
		t.Fatalf("storeBatch() error = %v", err)
	}

	if inserted != 2 {
		t.Errorf("storeBatch() = %d, want 2", inserted)
	}

	if tx.queued != 2 {
		t.Errorf("queued statements = %d, want 2", tx.queued)
	}

	if tx.commits != 1 {
		t.Errorf("commits = %d, want 1", tx.commits)
	}
}

func TestStoreBatchReportsExecutionFailure(t *testing.T) {
	tx := &fakeTransaction{results: &fakeBatchResults{execErr: errors.New("exec failed")}}

	inserted, err := storeBatch(context.Background(), tx, "INSERT", twoRecords())
	if err == nil {
		t.Fatal("storeBatch() error = nil, want the execution failure")
	}

	if inserted != 0 {
		t.Errorf("storeBatch() = %d, want 0", inserted)
	}

	if !errors.Is(err, tx.results.execErr) {
		t.Errorf("storeBatch() error = %v, want it to wrap the execution failure", err)
	}

	if tx.commits != 0 {
		t.Errorf("commits = %d, want none after a failed batch", tx.commits)
	}
}

func TestStoreBatchReportsCommitFailure(t *testing.T) {
	tx := &fakeTransaction{
		results:   &fakeBatchResults{affects: []int64{1}},
		commitErr: errors.New("commit failed"),
	}

	inserted, err := storeBatch(context.Background(), tx, "INSERT", twoRecords())
	if err == nil {
		t.Fatal("storeBatch() error = nil, want the commit failure")
	}

	if inserted != 0 {
		t.Errorf("storeBatch() = %d, want 0", inserted)
	}

	if !errors.Is(err, tx.commitErr) {
		t.Errorf("storeBatch() error = %v, want it to wrap the commit failure", err)
	}

	if tx.commits != 1 {
		t.Errorf("commits = %d, want 1", tx.commits)
	}
}

func twoRecords() []Anekdot {
	return []Anekdot{
		{CategoryID: 1, Text: "Первый анекдот"},
		{CategoryID: 1, Text: "Второй анекдот"},
	}
}
