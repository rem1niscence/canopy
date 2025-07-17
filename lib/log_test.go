package lib

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDefaultLogger(t *testing.T) {
	// pre-define the data-dir path for easy cleanup
	path := "./logger_test"
	// defer a simple cleanup of the path
	defer os.RemoveAll(path)
	// pre-define expected
	expected := NewLogger(LoggerConfig{
		Level: DebugLevel,
		Out:   os.Stdout,
	})
	// execute the function call
	got := NewDefaultLogger()
	// compare got vs expected
	require.Equal(t, got, expected)
}

func TestNewNullLogger(t *testing.T) {
	// pre-define the data-dir path for easy cleanup
	path := "./logger_test"
	// defer a simple cleanup of the path
	defer os.RemoveAll(path)
	// pre-define expected
	expected := NewLogger(LoggerConfig{
		Level: DebugLevel,
		Out:   io.Discard,
	})
	// execute the function call
	got := NewNullLogger()
	// compare got vs expected
	require.Equal(t, got, expected)
}

func TestParseErrorMessage(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		args           []any
		expectedMsg    string
		expectedFields []any
	}{
		{
			name:           "simple message without fields",
			format:         "error",
			args:           []any{},
			expectedMsg:    "error",
			expectedFields: []any{},
		},
		{
			name:           "message with colon suffix",
			format:         "controller error: ",
			args:           []any{},
			expectedMsg:    "controller error",
			expectedFields: []any{},
		},
		{
			name:        "message with single field",
			format:      "bft: \nheight: 123",
			args:        []any{},
			expectedMsg: "bft",
			expectedFields: []any{
				slog.String("height", "123"),
			},
		},
		{
			name:        "message with multiple fields",
			format:      "p2p peer failed: \nmodule: p2p\ncode: 20\nmessage: failure",
			args:        []any{},
			expectedMsg: "p2p peer failed",
			expectedFields: []any{
				slog.String("module", "p2p"),
				slog.String("code", "20"),
				slog.String("message", "failure"),
			},
		},
		{
			name:        "message with args formatting",
			format:      "peer %s failed: \nmodule: p2p\nip: %s",
			args:        []any{"localhost", "192.168.1.1"},
			expectedMsg: "peer localhost failed",
			expectedFields: []any{
				slog.String("module", "p2p"),
				slog.String("ip", "192.168.1.1"),
			},
		},
		{
			name:        "message with extra whitespace",
			format:      "bft wrong height: \n  module  :   bft  \n  code  :   400  ",
			args:        []any{},
			expectedMsg: "bft wrong height",
			expectedFields: []any{
				slog.String("module", "bft"),
				slog.String("code", "400"),
			},
		},
		{
			name:        "message with uppercase keys (should be lowercase)",
			format:      "P2P error: \nModule: P2P\nCode: 1",
			args:        []any{},
			expectedMsg: "P2P error",
			expectedFields: []any{
				slog.String("module", "P2P"),
				slog.String("code", "1"),
			},
		},
		{
			name:           "empty message",
			format:         "",
			args:           []any{},
			expectedMsg:    "",
			expectedFields: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, fields := parseErrorMessage(tt.format, tt.args...)
			if msg != tt.expectedMsg {
				t.Errorf("parseErrorMessage() msg = %v, want %v", msg, tt.expectedMsg)
			}
			require.Equal(t, fields, tt.expectedFields)
		})
	}
}
