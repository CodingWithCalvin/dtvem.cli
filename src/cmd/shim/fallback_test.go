package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
)

// The tests in this file run the shim as real processes. TestMain turns the
// test binary into the shim when envRunAsShim is set, and each test copies
// the test binary into place under the name a shim would have.
const (
	envRunAsShim = "DTVEM_TEST_RUN_AS_SHIM"
	// envFakeRealExe names the copy that acts as a real runtime executable
	// instead of a shim.
	envFakeRealExe = "DTVEM_TEST_FAKE_REAL_EXE"
	// envSpawnAgain makes the fake real executable run the command again
	// through PATH, once.
	envSpawnAgain = "DTVEM_TEST_SPAWN_AGAIN"
	envHops       = "DTVEM_TEST_HOPS"

	// maxHops bounds the process chain, so a regression fails the test
	// instead of exhausting the machine.
	maxHops         = 8
	exitTooManyHops = 99

	fakeRealReached = "fake real executable reached"
)

func TestMain(m *testing.M) {
	if os.Getenv(envRunAsShim) == "" {
		os.Exit(m.Run())
	}

	hops, _ := strconv.Atoi(os.Getenv(envHops))
	if hops >= maxHops {
		fmt.Fprintln(os.Stderr, "test guard: process chain too deep")
		os.Exit(exitTooManyHops)
	}
	_ = os.Setenv(envHops, strconv.Itoa(hops+1))

	if path.SameFile(selfPath(), os.Getenv(envFakeRealExe)) {
		os.Exit(runFakeRealExe())
	}

	main()
	os.Exit(0)
}

// runFakeRealExe stands in for a real runtime executable found on PATH and
// returns its exit code.
func runFakeRealExe() int {
	fmt.Printf("fallback marker names this executable: %v\n", path.SameFile(os.Getenv(fallbackEnvVar), selfPath()))

	if os.Getenv(envSpawnAgain) != "" {
		_ = os.Unsetenv(envSpawnAgain)
		cmd := exec.Command(shimNameFromPath(os.Args[0]))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode()
			}
			fmt.Fprintf(os.Stderr, "fake real executable could not run the command again: %v\n", err)
			return 1
		}
	}

	fmt.Println(fakeRealReached)
	return 0
}

// installTestBinary copies the running test binary to dir/name, with the
// .exe extension on Windows, and returns its path.
func installTestBinary(t *testing.T, dir, name string) string {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to locate test binary: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create %s: %v", dir, err)
	}
	if runtime.GOOS == constants.OSWindows {
		name += constants.ExtExe
	}
	dst := filepath.Join(dir, name)

	src, err := os.Open(self)
	if err != nil {
		t.Fatalf("Failed to open test binary: %v", err)
	}
	defer func() { _ = src.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		t.Fatalf("Failed to create %s: %v", dst, err)
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		t.Fatalf("Failed to copy test binary to %s: %v", dst, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close %s: %v", dst, err)
	}
	return dst
}

// runShimProcess runs shim with PATH set to pathDirs and DTVEM_ROOT set to an
// empty install, so no version resolves and the shim takes its PATH
// fallback. It returns the combined output and the exit code.
func runShimProcess(t *testing.T, shim string, pathDirs []string, extraEnv ...string) (string, int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, shim)
	cmd.Dir = t.TempDir()
	cmd.Env = append(shimTestEnv(),
		"PATH="+strings.Join(pathDirs, string(os.PathListSeparator)),
		"DTVEM_ROOT="+filepath.Join(t.TempDir(), "empty-root"),
		envRunAsShim+"=1",
	)
	cmd.Env = append(cmd.Env, extraEnv...)

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("shim did not exit within the timeout; output:\n%s", out)
	}
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Failed to run shim: %v", err)
	}
	return string(out), exitErr.ExitCode()
}

// shimTestEnv returns the current environment without the variables
// runShimProcess sets, or that would leak state from an earlier hop.
func shimTestEnv() []string {
	drop := []string{"PATH", "DTVEM_ROOT", "XDG_DATA_HOME", fallbackEnvVar, envRunAsShim, envFakeRealExe, envSpawnAgain, envHops}

	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		keep := true
		for _, d := range drop {
			if strings.EqualFold(key, d) {
				keep = false
				break
			}
		}
		if keep {
			env = append(env, kv)
		}
	}
	return env
}

func assertNoRunawayChain(t *testing.T, out string, code int) {
	t.Helper()
	if code == exitTooManyHops || strings.Contains(out, "test guard") {
		t.Fatalf("shim kept launching processes (exit %d); output:\n%s", code, out)
	}
}

func TestShimFallback_SkipsOwnDirectoryWhenHomeDiffers(t *testing.T) {
	// The shims directory on PATH is not the one DTVEM_ROOT names, as when a
	// sandbox redirects the home directory. Before the fix the shim found
	// itself on PATH and launched itself forever.
	shimsDir := filepath.Join(t.TempDir(), "install", "shims")
	shim := installTestBinary(t, shimsDir, "node")

	out, code := runShimProcess(t, shim, []string{shimsDir})

	assertNoRunawayChain(t, out, code)
	if code != 1 {
		t.Errorf("exit code = %d, want 1; output:\n%s", code, out)
	}
	if !strings.Contains(out, "No system installation found in PATH") {
		t.Errorf("output does not report a missing system install:\n%s", out)
	}
}

func TestShimFallback_RunsRealExecutableFurtherDownPath(t *testing.T) {
	tempDir := t.TempDir()
	shimsDir := filepath.Join(tempDir, "install", "shims")
	systemDir := filepath.Join(tempDir, "system")
	shim := installTestBinary(t, shimsDir, "node")
	realExe := installTestBinary(t, systemDir, "node")

	out, code := runShimProcess(t, shim, []string{shimsDir, systemDir}, envFakeRealExe+"="+realExe)

	assertNoRunawayChain(t, out, code)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; output:\n%s", code, out)
	}
	if !strings.Contains(out, fakeRealReached) {
		t.Errorf("real executable was not reached; output:\n%s", out)
	}
	if !strings.Contains(out, "fallback marker names this executable: true") {
		t.Errorf("real executable did not receive %s naming it; output:\n%s", fallbackEnvVar, out)
	}
}

func TestShimFallback_StopsWhenFallbackIsAnotherShim(t *testing.T) {
	// Two shim directories on PATH, neither the one DTVEM_ROOT names. The
	// first shim skips its own directory and finds the second; without the
	// guard the two would launch each other forever.
	tempDir := t.TempDir()
	shimsA := filepath.Join(tempDir, "a", "shims")
	shimsB := filepath.Join(tempDir, "b", "shims")
	shim := installTestBinary(t, shimsA, "node")
	installTestBinary(t, shimsB, "node")

	out, code := runShimProcess(t, shim, []string{shimsA, shimsB})

	assertNoRunawayChain(t, out, code)
	if code != 1 {
		t.Errorf("exit code = %d, want 1; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Refusing to run 'node'") {
		t.Errorf("output does not report the fallback loop:\n%s", out)
	}
}

func TestShimFallback_RealExecutableMayRunTheCommandAgain(t *testing.T) {
	// A real node reached through the fallback starts `node` again, as a
	// script spawning node does. That second shim inherits the fallback
	// marker but is not the file it names, so it must run normally.
	tempDir := t.TempDir()
	shimsDir := filepath.Join(tempDir, "install", "shims")
	systemDir := filepath.Join(tempDir, "system")
	shim := installTestBinary(t, shimsDir, "node")
	realExe := installTestBinary(t, systemDir, "node")

	out, code := runShimProcess(t, shim, []string{shimsDir, systemDir},
		envFakeRealExe+"="+realExe,
		envSpawnAgain+"=1",
	)

	assertNoRunawayChain(t, out, code)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; output:\n%s", code, out)
	}
	if strings.Contains(out, "Refusing to run") {
		t.Errorf("nested launch was mistaken for a fallback loop; output:\n%s", out)
	}
	if got := strings.Count(out, fakeRealReached); got != 2 {
		t.Errorf("real executable reached %d times, want 2; output:\n%s", got, out)
	}
}
