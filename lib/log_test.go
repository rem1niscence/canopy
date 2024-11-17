package lib

import (
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
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
