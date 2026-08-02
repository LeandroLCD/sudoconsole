package cli

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/agent"
)

func TestDetectCmd_JSON(t *testing.T) {
	fsys := fstest.MapFS{
		"home/u/.claude": &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		"home/u/.aider":  &fstest.MapFile{Mode: fs.ModeDir | 0o755},
	}
	d := agent.NewDetector()
	d.FS = fsys
	d.UserHomeDir = func() (string, error) { return "/home/u", nil }
	d.Timeout = 500 * time.Millisecond
	// Force the detector to ignore any real host binaries.
	d.PathLayout = func() []string { return nil }

	app := stubApp(t)
	app.AgentDetector = d
	fmtImpl, _ := NewFormatter(FormatJSON, false)
	app.Formatter = fmtImpl

	stdout, _, err := cmdFromArgs(app, []string{"detect"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got DetectResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v body=%q", err, stdout)
	}
	if len(got.Detected) != 2 {
		t.Errorf("want 2 detections, got %d: %+v", len(got.Detected), got.Detected)
	}
}

func TestDetectCmd_HumanEmpty(t *testing.T) {
	app := stubApp(t)
	app.AgentDetector = &emptyDetector{}
	stdout, _, err := cmdFromArgs(app, []string{"detect"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "no supported agents found") {
		t.Errorf("expected empty message; got %q", stdout)
	}
}

func TestDetectCmd_HumanPopulated(t *testing.T) {
	fsys := fstest.MapFS{
		"home/u/.config/kilo": &fstest.MapFile{Mode: fs.ModeDir | 0o755},
	}
	d := agent.NewDetector()
	d.FS = fsys
	d.UserHomeDir = func() (string, error) { return "/home/u", nil }
	d.PathLayout = func() []string { return nil }

	app := stubApp(t)
	app.AgentDetector = d
	fmtImpl, _ := NewFormatter(FormatHuman, false)
	app.Formatter = fmtImpl
	stdout, _, err := cmdFromArgs(app, []string{"detect"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "kilo") || !strings.Contains(stdout, "Kilo CLI") {
		t.Errorf("expected kilo entry; got %q", stdout)
	}
}

// emptyDetector is a detector that returns no agents.
type emptyDetector struct{}

func (emptyDetector) Detect(_ context.Context) ([]domain.AgentDescriptor, error) {
	return nil, nil
}

func (emptyDetector) DetectOne(_ context.Context, _ domain.AgentKind) (domain.AgentDescriptor, error) {
	return domain.AgentDescriptor{}, domain.ErrAgentNotDetected
}

// silence unused import warnings.
var _ = time.Second
