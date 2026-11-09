package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	stubdatabase "github.com/golang-migrate/migrate/v4/database/stub"
	"github.com/golang-migrate/migrate/v4/source"
	stubsource "github.com/golang-migrate/migrate/v4/source/stub"
)

func resetFlags(t *testing.T) {
	t.Helper()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	original := os.Args
	t.Cleanup(func() { os.Args = original })
}

func setBaseEnv(t *testing.T) {
	t.Helper()

	const dsn = "postgres://user:pass@localhost:5432/app"

	for key, value := range map[string]string{
		"HTTP_ADDR":             ":8080",
		"DATABASE_URL":          dsn,
		"DB_MAX_CONNS":          "4",
		"DB_MIN_CONNS":          "0",
		"DB_MAX_CONN_LIFETIME":  "30m",
		"DB_MAX_CONN_IDLE_TIME": "5m",
		"REQUEST_TIMEOUT":       "10s",
		"SHUTDOWN_TIMEOUT":      "15s",
		"SESSION_TTL":           "720h",
		"LOG_FORMAT":            "json",
		"LOG_LEVEL":             "info",
		"WEB_DIR":               "backend/web",
		"DATA_DIR":              "data",
		"EXTERNAL_ANEKDOT_URL":  "https://example.com/random",
		"CORS_ORIGINS":          "http://localhost:5173",
		"TRUST_PROXY_HEADERS":   "false",
	} {
		t.Setenv(key, value)
	}
}

func TestParseOptionsDefaults(t *testing.T) {
	resetFlags(t)
	os.Args = []string{"migrate"}

	opts := parseOptions()
	if opts.direction != directionUp {
		t.Fatalf("direction = %q, want %q", opts.direction, directionUp)
	}

	if opts.steps != 0 {
		t.Fatalf("steps = %d, want 0", opts.steps)
	}
}

func TestParseOptionsReadsFlags(t *testing.T) {
	resetFlags(t)
	os.Args = []string{"migrate", "-direction=down", "-steps=3"}

	opts := parseOptions()
	if opts.direction != directionDown {
		t.Fatalf("direction = %q, want %q", opts.direction, directionDown)
	}

	if opts.steps != 3 {
		t.Fatalf("steps = %d, want 3", opts.steps)
	}
}

func TestRunRejectsUnknownDirection(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "typo", args: []string{"-direction=sideways"}},
		{name: "empty", args: []string{"-direction="}},
		{name: "opposite case", args: []string{"-direction=UP"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetFlags(t)
			setBaseEnv(t)
			os.Args = append([]string{"migrate"}, test.args...)

			err := run()
			if err == nil {
				t.Fatal("run() error = nil, want error")
			}

			if !strings.Contains(err.Error(), "unsupported direction") {
				t.Fatalf("run() error = %q, want it to mention the direction", err)
			}
		})
	}
}

func TestRunPropagatesInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{
			name: "wrong scheme",
			env:  map[string]string{"DATABASE_URL": "mysql://user:pass@localhost:3306/app"},
			want: "DATABASE_URL",
		},
		{
			name: "missing database name",
			args: []string{"-direction=down"},
			env:  map[string]string{"DATABASE_URL": "postgres://user:pass@localhost:5432"},
			want: "database name",
		},
		{
			name: "broken address",
			args: []string{"-direction=version"},
			env: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost:5432/app",
				"HTTP_ADDR":    "not-a-host-port",
			},
			want: "HTTP_ADDR",
		},
		{
			name: "swapped port bounds",
			args: []string{"-direction=up", "-steps=2"},
			env: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost:5432/app",
				"DB_MAX_CONNS": "1",
				"DB_MIN_CONNS": "5",
			},
			want: "DB_MIN_CONNS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetFlags(t)
			setBaseEnv(t)
			os.Args = append([]string{"migrate"}, test.args...)

			for key, value := range test.env {
				t.Setenv(key, value)
			}

			err := run()
			if err == nil {
				t.Fatal("run() error = nil, want error")
			}

			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run() error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestRunAcceptsEveryDirection(t *testing.T) {
	for _, direction := range []string{directionUp, directionDown, directionVersion} {
		t.Run(direction, func(t *testing.T) {
			resetFlags(t)
			setBaseEnv(t)
			os.Args = []string{"migrate", "-direction=" + direction}
			t.Setenv("DATABASE_URL", "ftp://user:pass@localhost:5432/app")

			err := run()
			if err == nil {
				t.Fatal("run() error = nil, want the configuration error")
			}

			if strings.Contains(err.Error(), "unsupported direction") {
				t.Fatalf("run() rejected the known direction %q", direction)
			}
		})
	}
}

func TestRunAcceptsEveryDirectionWithoutDatabase(t *testing.T) {
	for _, args := range [][]string{
		{"-direction=up"},
		{"-direction=down", "-steps=2"},
		{"-direction=version"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			resetFlags(t)
			setBaseEnv(t)
			os.Args = append([]string{"migrate"}, args...)
			t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/app")

			err := run()
			if err == nil {
				t.Fatal("run() error = nil, want a connection error")
			}

			if !strings.Contains(err.Error(), "pinging database") {
				t.Fatalf("run() error = %q, want it to mention pinging", err)
			}
		})
	}
}

func newStubMigrator(t *testing.T, currentVersion int) *migrate.Migrate {
	t.Helper()

	migrations := source.NewMigrations()
	for _, version := range []uint{1, 2} {
		label := fmt.Sprint(version)
		migrations.Append(&source.Migration{Version: version, Identifier: label, Direction: source.Up})
		migrations.Append(&source.Migration{Version: version, Identifier: label, Direction: source.Down})
	}

	databaseDriver, err := stubdatabase.WithInstance(nil, &stubdatabase.Config{})
	if err != nil {
		t.Fatalf("creating database stub: %v", err)
	}

	if err := databaseDriver.SetVersion(currentVersion, false); err != nil {
		t.Fatalf("setting version: %v", err)
	}

	return newMigrator(t, &stubsource.Stub{Migrations: migrations}, databaseDriver)
}

func newMigrator(t *testing.T, sourceDriver source.Driver, databaseDriver database.Driver) *migrate.Migrate {
	t.Helper()

	migrator, err := migrate.NewWithInstance("stub", sourceDriver, "stub", databaseDriver)
	if err != nil {
		t.Fatalf("creating migrator: %v", err)
	}

	return migrator
}

func migratorVersion(t *testing.T, migrator *migrate.Migrate) int {
	t.Helper()

	version, _, err := migrator.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return database.NilVersion
	}

	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}

	return int(version)
}

func TestApplyDirections(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion int
		direction      string
		steps          int
		wantVersion    int
	}{
		{name: "up applies everything", currentVersion: database.NilVersion, direction: directionUp, wantVersion: 2},
		{name: "down keeps one step", currentVersion: 2, direction: directionDown, steps: 1, wantVersion: 1},
		{name: "down keeps both steps", currentVersion: 2, direction: directionDown, steps: 2, wantVersion: database.NilVersion},
		{name: "down without steps", currentVersion: 2, direction: directionDown, wantVersion: database.NilVersion},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			migrator := newStubMigrator(t, test.currentVersion)

			if err := apply(migrator, test.direction, test.steps); err != nil {
				t.Fatalf("apply() error = %v", err)
			}

			if version := migratorVersion(t, migrator); version != test.wantVersion {
				t.Errorf("version = %d, want %d", version, test.wantVersion)
			}
		})
	}
}

func TestApplyReportsNoChange(t *testing.T) {
	migrator := newStubMigrator(t, 2)

	if err := apply(migrator, directionUp, 0); !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("apply(up) error = %v, want ErrNoChange", err)
	}
}

func TestApplyRejectsUnknownDirection(t *testing.T) {
	err := apply(nil, "sideways", 0)
	if err == nil {
		t.Fatal("apply() error = nil, want an error for an unknown direction")
	}

	if !strings.Contains(err.Error(), "unsupported direction") {
		t.Fatalf("apply() error = %q, want it to reject the direction", err)
	}
}

func TestApplyVersion(t *testing.T) {
	migrator := newStubMigrator(t, 1)

	if err := apply(migrator, directionVersion, 0); err != nil {
		t.Fatalf("apply(version) error = %v", err)
	}
}

func TestPrintVersion(t *testing.T) {
	t.Run("applied", func(t *testing.T) {
		if err := printVersion(newStubMigrator(t, 2)); err != nil {
			t.Fatalf("printVersion() error = %v", err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		if err := printVersion(newStubMigrator(t, database.NilVersion)); err != nil {
			t.Fatalf("printVersion() error = %v", err)
		}
	})
}

type brokenVersionDriver struct {
	*stubdatabase.Stub
}

func (d *brokenVersionDriver) Version() (int, bool, error) {
	return 0, false, errors.New("version is broken")
}

func TestPrintVersionReportsDriverError(t *testing.T) {
	sourceDriver := &stubsource.Stub{Migrations: source.NewMigrations()}
	migrator := newMigrator(t, sourceDriver, &brokenVersionDriver{})

	err := printVersion(migrator)
	if err == nil {
		t.Fatal("printVersion() error = nil, want the driver error")
	}

	if !strings.Contains(err.Error(), "reading migration version") {
		t.Fatalf("printVersion() error = %q, want it to mention reading the version", err)
	}
}

func TestValidateDirection(t *testing.T) {
	for _, direction := range []string{directionUp, directionDown, directionVersion} {
		t.Run("accepts "+direction, func(t *testing.T) {
			if err := validateDirection(direction); err != nil {
				t.Fatalf("validateDirection(%q) error = %v", direction, err)
			}
		})
	}

	for _, direction := range []string{"", "sideways", "UP"} {
		t.Run("rejects "+direction, func(t *testing.T) {
			err := validateDirection(direction)
			if err == nil {
				t.Fatalf("validateDirection(%q) error = nil, want error", direction)
			}

			if !strings.Contains(err.Error(), "unsupported direction") {
				t.Fatalf("validateDirection(%q) error = %q, want it to mention the direction", direction, err)
			}
		})
	}
}

func TestOpenDatabaseRejectsUnreachableServer(t *testing.T) {
	db, err := openDatabase("postgres://user:pass@127.0.0.1:1/app")
	if err == nil {
		_ = db.Close()
		t.Fatal("openDatabase() error = nil, want a connection error")
	}

	if db != nil {
		t.Errorf("openDatabase() database = %v, want nil", db)
	}

	if !strings.Contains(err.Error(), "pinging database") {
		t.Fatalf("openDatabase() error = %q, want it to mention pinging", err)
	}
}

func TestOpenDatabaseRejectsInvalidDSN(t *testing.T) {
	db, err := openDatabase("postgres://user:pass@localhost:5432/app?sslmode=bogus")
	if err == nil {
		_ = db.Close()
		t.Fatal("openDatabase() error = nil, want a configuration error")
	}

	if db != nil {
		t.Errorf("openDatabase() database = %v, want nil", db)
	}

	if !strings.Contains(err.Error(), "database") {
		t.Fatalf("openDatabase() error = %q, want it to mention the database", err)
	}
}

func TestBuildMigratorRejectsUnreachableDatabase(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://user:pass@127.0.0.1:1/app")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	migrator, err := buildMigrator(db, fstest.MapFS{})
	if err == nil {
		_, _ = migrator.Close()
		t.Fatal("buildMigrator() error = nil, want a driver error")
	}

	if migrator != nil {
		t.Errorf("buildMigrator() migrator = %v, want nil", migrator)
	}

	if !strings.Contains(err.Error(), "creating migration driver") {
		t.Fatalf("buildMigrator() error = %q, want it to mention the driver", err)
	}
}

func TestBuildMigratorRejectsDuplicateSource(t *testing.T) {
	files := fstest.MapFS{
		"000001_first.up.sql":  {Data: []byte("select 1")},
		"000001_second.up.sql": {Data: []byte("select 2")},
	}

	migrator, err := buildMigrator(nil, files)
	if err == nil {
		_, _ = migrator.Close()
		t.Fatal("buildMigrator() error = nil, want a source error")
	}

	if migrator != nil {
		t.Errorf("buildMigrator() migrator = %v, want nil", migrator)
	}

	if !strings.Contains(err.Error(), "creating migration source") {
		t.Fatalf("buildMigrator() error = %q, want it to mention the source", err)
	}
}

func TestFinish(t *testing.T) {
	t.Run("clean run", func(t *testing.T) {
		if err := finish(nil, directionUp); err != nil {
			t.Fatalf("finish(nil) error = %v", err)
		}
	})

	t.Run("no change", func(t *testing.T) {
		if err := finish(migrate.ErrNoChange, directionVersion); err != nil {
			t.Fatalf("finish(ErrNoChange) error = %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cause := errors.New("boom")

		err := finish(cause, directionDown)
		if err == nil {
			t.Fatal("finish(error) error = nil, want error")
		}

		if !strings.Contains(err.Error(), "applying migrations") {
			t.Fatalf("finish() error = %q, want it to mention applying migrations", err)
		}

		if !errors.Is(err, cause) {
			t.Fatalf("finish() error = %q, want it to wrap the cause", err)
		}
	})
}
