package entrypoint

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

const subprocessEnv = "ANEKDOTUS_ENTRYPOINT_FAIL"

func TestRunCallsTheAction(t *testing.T) {
	called := false

	Run("should not be logged", func() error {
		called = true

		return nil
	})

	if !called {
		t.Fatal("action was not called")
	}
}

func TestRunExitsOnFailure(t *testing.T) {
	if os.Getenv(subprocessEnv) == "1" {
		Run("failure during test", func() error { return errors.New("boom") })

		return
	}

	//nolint:gosec // re-executing the test binary is the standard os.Exit pattern
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestRunExitsOnFailure")
	cmd.Env = append(os.Environ(), subprocessEnv+"=1")

	var exitErr *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exitErr) {
		t.Fatalf("expected the subprocess to exit with an error, got %v", err)
	}
	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
