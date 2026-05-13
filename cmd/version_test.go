package cmd

import (
	"bytes"
	"testing"

	"github.com/xrmcp/go-sdk/xrmcp"
)

func TestVersionInfoFormat(t *testing.T) {
	original := cliVersion
	cliVersion = "0.1.0"
	t.Cleanup(func() { cliVersion = original })

	want := "spec: " + xrmcp.SpecVersion + "\nxrmcp/go-sdk: " + xrmcp.SDKVersion + "\nxrmcp/cli: 0.1.0\n"
	if got := versionInfo(); got != want {
		t.Fatalf("unexpected version info:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestEffectiveCLIVersionFallsBackToDev(t *testing.T) {
	original := cliVersion
	cliVersion = ""
	t.Cleanup(func() { cliVersion = original })

	if got := effectiveCLIVersion(); got != "dev" {
		t.Fatalf("expected dev fallback, got %q", got)
	}
}

func TestRootVersionFlagMatchesVersionCommand(t *testing.T) {
	original := cliVersion
	cliVersion = "0.1.0"
	t.Cleanup(func() { cliVersion = original })

	versionOutput := executeRootForVersionTest(t, "version")
	flagOutput := executeRootForVersionTest(t, "--version")
	shortFlagOutput := executeRootForVersionTest(t, "-v")

	if versionOutput != flagOutput {
		t.Fatalf("--version output mismatch:\nversion:\n%s\n--version:\n%s", versionOutput, flagOutput)
	}
	if versionOutput != shortFlagOutput {
		t.Fatalf("-v output mismatch:\nversion:\n%s\n-v:\n%s", versionOutput, shortFlagOutput)
	}
}

func executeRootForVersionTest(t *testing.T, args ...string) string {
	t.Helper()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	versionFlag = false

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(%v): %v", args, err)
	}

	return buf.String()
}
