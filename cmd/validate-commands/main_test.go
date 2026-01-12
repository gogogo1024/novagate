package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecimalCmdDef_IsReportedWithoutCascadeNoise(t *testing.T) {
	tmp := t.TempDir()
	commandsPath := filepath.Join(tmp, "commands.go")
	serverPath := filepath.Join(tmp, "server.go")
	registryPath := filepath.Join(tmp, "registry.go")

	// Intentionally define command in decimal with a trailing comment.
	mustWrite(t, commandsPath, "package protocol\n\nconst (\n\tCmdFoo uint16 = 123 // should be hex\n)\n")

	// These files are parsed as plain text by the validator; they don't need to compile.
	mustWrite(t, serverPath, `package main

import "github.com/gogogo1024/novagate/protocol"

func setup() {
	protocol.RegisterFullMethodCommand("Foo.Bar", protocol.CmdFoo)
	bridge(protocol.CmdFoo)
}
`)

	mustWrite(t, registryPath, `package service

import (
	"github.com/gogogo1024/novagate/internal/dispatcher"
	"github.com/gogogo1024/novagate/protocol"
)

func init() {
	dispatcher.Register(protocol.CmdFoo, nil)
}
`)

	scan, scanIssues := parseFixed(commandsPath, serverPath, registryPath)
	issues := append([]issue{}, scanIssues...)
	issues = append(issues, validateConsistency(scan, false)...)

	joined := joinIssues(issues)

	if !strings.Contains(joined, "must be a hex literal") {
		t.Fatalf("expected hex-literal error, got:\n%s", joined)
	}
	if strings.Contains(joined, "unknown command protocol.CmdFoo") {
		t.Fatalf("expected no cascade 'unknown command' noise, got:\n%s", joined)
	}
	if strings.Contains(joined, "no Cmd* uint16 constants found") {
		t.Fatalf("expected no 'no Cmd*' noise when decimal defs exist, got:\n%s", joined)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func joinIssues(issues []issue) string {
	var b strings.Builder
	for _, it := range issues {
		b.WriteString(it.msg)
		b.WriteByte('\n')
	}
	return b.String()
}
