package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	runTest := os.Getenv("INTEGRATION_TEST")
	if runTest == "" {
		t.Skip("set INTEGRATION_TEST to run this test")
	}

	testCases := map[string]struct {
		diff       bool
		args       []string
		wantUntidy bool
		wantErr    bool
	}{
		"multiple modules, including untidy": {
			diff: true,
			args: []string{
				"testdata/untidy/module1",
				"testdata/untidy/module2",
				"testdata/tidy/module3",
			},
			wantUntidy: true,
		},
		"empty args defaults to current directory": {
			diff:       true,
			args:       []string{},
			wantUntidy: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := context.Background()
			flags := flags{
				diff:    &tc.diff,
				verbose: truePtr(),
				version: falsePtr(),
			}

			untidy, err := run(ctx, flags, tc.args)

			if tc.wantErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
			assert.Equal(tc.wantUntidy, untidy)
		})
	}
}

func TestCheck(t *testing.T) {
	runTest := os.Getenv("INTEGRATION_TEST")
	if runTest == "" {
		t.Skip("set INTEGRATION_TEST to run this test")
	}

	testCases := map[string]struct {
		path       string
		wantUntidy bool
		wantErr    bool
	}{
		"module1 is not tidy": {
			path:       "testdata/untidy/module1",
			wantUntidy: true,
		},
		"module2 is not tidy": {
			path:       "testdata/untidy/module2",
			wantUntidy: true,
		},
		"module3 is tidy": {
			path: "testdata/tidy/module3",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := context.Background()
			logger := debugLogger{}

			untidy, err := check(ctx, tc.path, true, logger)

			if tc.wantErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
			assert.Equal(tc.wantUntidy, untidy)
		})
	}
}

func truePtr() *bool {
	b := true
	return &b
}

func falsePtr() *bool {
	b := false
	return &b
}
