package cmdexec

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"testing"
)

const (
	envFakeStdout   = "CMDEXEC_FAKE_STDOUT"
	envFakeStderr   = "CMDEXEC_FAKE_STDERR"
	envFakeExitCode = "CMDEXEC_FAKE_EXITCODE"
)

var flagSet = flag.NewFlagSet("", flag.ContinueOnError)
var flagMockbin = flagSet.Bool("mockbin", false, "Use this flag to mock a binary")

func IsMockbin() bool {
	if !flagSet.Parsed() {
		// By default the flagSet uses /dev/stderr, so flag errors are visible
		// when executing tests based on IsMockbin(). These errors always happen
		// as we pass the whole os.Args[1:] to Parse().
		flagSet.SetOutput(&bytes.Buffer{})
		_ = flagSet.Parse(os.Args[1:])
	}

	return *flagMockbin
}

func Mockbin() {
	if out, ok := os.LookupEnv(envFakeStdout); ok {
		fmt.Fprint(os.Stdout, out)
	}

	if err, ok := os.LookupEnv(envFakeStderr); ok {
		fmt.Fprint(os.Stderr, err)
	}

	if stringCode, ok := os.LookupEnv(envFakeExitCode); ok {
		code, err := strconv.Atoi(stringCode)
		if err != nil {
			panic(err)
		}
		os.Exit(code)
	}

	os.Exit(0)
}

type Recorder struct {
	executions []exec.Cmd
}

func (r *Recorder) recordExecution(cmd *exec.Cmd) {
	r.executions = append(r.executions, *cmd)
}

// NewTestExecutor creates a command executor for testing purpose. Two ExecOpt
// are added to the executor options: 1. to prefix the command args to mock the
// command execution using Mockbin() and 2. to record all the command executed
// (before they're mutated by the first option).
func NewTestExecutor() (CmdExecutor, *Recorder) {
	recorder := &Recorder{
		executions: []exec.Cmd{},
	}
	executor := NewExecutor(
		prefixCmdArgsWithMockbin,
		recorder.recordExecution)

	return executor, recorder
}

func prefixCmdArgsWithMockbin(cmd *exec.Cmd) {
	// Create a new exec.Cmd to ignore any lookPathErr (happens when the
	// mocked bin isn't installed).
	new := exec.Command(os.Args[0], "-mockbin")
	new.Env = cmd.Env
	new.Dir = cmd.Dir
	new.Stdout = cmd.Stdout
	new.Stderr = cmd.Stderr

	*cmd = *new
}

type Tester func(*testing.T, *Recorder)

func BuildTesters(testers ...Tester) Tester {
	return func(t *testing.T, r *Recorder) {
		for _, tester := range testers {
			tester(t, r)
		}
	}
}

func ExpectCommandArgs(expectedArgs []string) Tester {
	return func(t *testing.T, r *Recorder) {
		for _, execution := range r.executions {
			if reflect.DeepEqual(expectedArgs, execution.Args) {
				return
			}
		}

		t.Fatalf("There're no command execution recorded with args: %v",
			expectedArgs)
	}
}

func FakeOn(expectedArgs []string, opts ...ExecOpt) ExecOpt {
	return func(cmd *exec.Cmd) {
		if !reflect.DeepEqual(cmd.Args, expectedArgs) {
			return
		}

		for _, opt := range opts {
			opt(cmd)
		}
	}
}

func FakeStdout(out string) ExecOpt {
	return ExtraEnv([]string{
		fmt.Sprintf("%s=%s", envFakeStdout, out),
	})
}

func FakeStderr(err string) ExecOpt {
	return ExtraEnv([]string{
		fmt.Sprintf("%s=%s", envFakeStderr, err),
	})
}

func FakeExitCode(exitCode int) ExecOpt {
	return ExtraEnv([]string{
		fmt.Sprintf("%s=%s", envFakeExitCode, strconv.Itoa(exitCode)),
	})
}
