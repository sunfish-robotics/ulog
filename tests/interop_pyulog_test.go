//go:build pyulog

package tests

import "testing"

func TestPyULogReadsGoAndGoReadsPyULog(t *testing.T) {
	RunPyULogTest(t)
}
