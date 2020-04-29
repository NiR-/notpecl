package cmdexec

import (
	"io"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

type CmdExecutor struct {
	opts []ExecOpt
}

type ExecOpt func(*exec.Cmd)

// NewExecutor creates a new CmdExecutor with the given list of ExecOpt
// reversed. In this way, the first opt is executed at last.
func NewExecutor(opts ...ExecOpt) CmdExecutor {
	for i, j := 0, len(opts)-1; i < j; i, j = i+1, j-1 {
		opts[i], opts[j] = opts[j], opts[i]
	}

	return CmdExecutor{opts}
}

// With returns a new CmdExecutor with the given ExecOpt prepended to the list
// of ExecOpt of the current executor. In this way, the options added first are
// executed at last.
func (executor CmdExecutor) With(opts ...ExecOpt) CmdExecutor {
	return CmdExecutor{append(opts, executor.opts...)}
}

// Run creates a new exec.Cmd and applies the ExecOpt of the executor and then
// run the Cmd.
func (executor CmdExecutor) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	for _, opt := range executor.opts {
		opt(cmd)
	}

	logrus.Debugf("Running %s...", strings.Join(cmd.Args, " "))
	return cmd.Run()
}

// BaseDir returns an ExecOpt that sets the Dir field of the exec.Cmd.
func BaseDir(basedir string) ExecOpt {
	return func(cmd *exec.Cmd) {
		cmd.Dir = basedir
	}
}

// Stdout returns an ExecOpt that sets the Stdout field of the exec.Cmd.
func Stdout(w io.Writer) ExecOpt {
	return func(cmd *exec.Cmd) {
		cmd.Stdout = w
	}
}

// Stderr returns an ExecOpt that sets the Stderr field of the exec.Cmd.
func Stderr(w io.Writer) ExecOpt {
	return func(cmd *exec.Cmd) {
		cmd.Stderr = w
	}
}

// ExtraEnv returns an ExecOpt that sets the Env field of the exec.Cmd.
func ExtraEnv(env []string) ExecOpt {
	return func(cmd *exec.Cmd) {
		cmd.Env = append(cmd.Env, env...)
	}
}
