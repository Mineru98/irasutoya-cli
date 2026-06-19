package preview

import (
	"errors"
	"reflect"
	"testing"
)

func TestOpenerUsesPlatformCommand(t *testing.T) {
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{
			goos:     "windows",
			wantName: "rundll32",
			wantArgs: []string{"url.dll,FileProtocolHandler", "https://example.com/image.png"},
		},
		{
			goos:     "darwin",
			wantName: "open",
			wantArgs: []string{"https://example.com/image.png"},
		},
		{
			goos:     "linux",
			wantName: "xdg-open",
			wantArgs: []string{"https://example.com/image.png"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			var gotName string
			var gotArgs []string
			opener := NewOpenerForOS(tt.goos, func(name string, args ...string) error {
				gotName = name
				gotArgs = args
				return nil
			})

			if err := opener.ShowURL("https://example.com/image.png"); err != nil {
				t.Fatalf("ShowURL returned error: %v", err)
			}
			if gotName != tt.wantName {
				t.Fatalf("command mismatch: want %q, got %q", tt.wantName, gotName)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("args mismatch: want %v, got %v", tt.wantArgs, gotArgs)
			}
		})
	}
}

func TestOpenerReturnsRunnerError(t *testing.T) {
	runnerErr := errors.New("runner failed")
	opener := NewOpenerForOS("linux", func(string, ...string) error {
		return runnerErr
	})

	if err := opener.ShowURL("https://example.com/image.png"); !errors.Is(err, runnerErr) {
		t.Fatalf("expected runner error %v, got %v", runnerErr, err)
	}
}

func TestOpenerRejectsUnsupportedPlatform(t *testing.T) {
	opener := NewOpenerForOS("plan9", func(string, ...string) error {
		t.Fatal("runner should not be called for unsupported platform")
		return nil
	})

	if err := opener.ShowURL("https://example.com/image.png"); err == nil {
		t.Fatal("expected unsupported platform error")
	}
}
