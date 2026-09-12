package main

import (
	"testing"

	"github.com/khanakia/browserctrl/browser/browsertest"
)

// writeStore is a thin alias so the CLI tests read like the browser tests.
func writeStore(t *testing.T, dir string, kv map[string]any) {
	t.Helper()
	browsertest.WriteStore(t, dir, kv)
}
