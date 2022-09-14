package main

import "fmt"

type logger interface {
	Log(format string, args ...interface{})
}

type nopLogger struct{}

func (nopLogger) Log(format string, args ...interface{}) {}

type debugLogger struct{}

func (debugLogger) Log(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}
