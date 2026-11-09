package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, action func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = writer

	action()

	if err := writer.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}
	os.Stdout = original

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading pipe: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("closing reader: %v", err)
	}

	return string(data)
}

func TestNewSelectsHandlerByFormat(t *testing.T) {
	cases := []struct {
		name   string
		level  slog.Level
		format string
		want   string
	}{
		{name: "json", level: slog.LevelInfo, format: "json", want: "*slog.JSONHandler"},
		{name: "text", level: slog.LevelInfo, format: "text", want: "*slog.TextHandler"},
		{name: "unknown falls back to json", level: slog.LevelWarn, format: "yaml", want: "*slog.JSONHandler"},
		{name: "uppercase text is not text", level: slog.LevelInfo, format: "TEXT", want: "*slog.JSONHandler"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := New(testCase.level, testCase.format)
			if got := fmt.Sprintf("%T", logger.Handler()); got != testCase.want {
				t.Fatalf("handler = %s, want %s", got, testCase.want)
			}
		})
	}
}

func TestNewAddsSourceOnlyInDebug(t *testing.T) {
	debugOutput := captureStdout(t, func() {
		New(slog.LevelDebug, "json").Debug("probe")
	})
	if !strings.Contains(debugOutput, `"source"`) {
		t.Fatalf("debug output has no source attribute: %s", debugOutput)
	}

	infoOutput := captureStdout(t, func() {
		New(slog.LevelInfo, "json").Info("probe")
	})
	if strings.Contains(infoOutput, `"source"`) {
		t.Fatalf("info output must not contain a source attribute: %s", infoOutput)
	}
}

func TestNewWritesTextFormat(t *testing.T) {
	output := captureStdout(t, func() {
		New(slog.LevelInfo, "text").Info("probe", "field", "value")
	})
	if !strings.Contains(output, "msg=probe") || !strings.Contains(output, "field=value") {
		t.Fatalf("unexpected text output: %s", output)
	}
}
