package pecl

import (
	"context"
	"io"
	"os"
	"os/exec"
)

type CmdRunner struct {
	basedir    string
	out        io.Writer
	err        io.Writer
	env        []string
	cmdMutator CmdMutator
}

type CmdMutator func(string, []string) (string, []string)

func NewCmdRunner() CmdRunner {
	return CmdRunner{
		out: os.Stdout,
		err: os.Stderr,
	}
}

func (r CmdRunner) WithBaseDir(basedir string) CmdRunner {
	r.basedir = basedir
	return r
}

func (r CmdRunner) WithOutWriter(w io.Writer) CmdRunner {
	r.out = w
	return r
}

func (r CmdRunner) WithExtraEnv(env []string) CmdRunner {
	r.env = append(r.env, env...)
	return r
}

func (r CmdRunner) WithCmdMutator(mutator CmdMutator) CmdRunner {
	r.cmdMutator = mutator
	return r
}

func (r CmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	if r.cmdMutator != nil {
		cmd, args = r.cmdMutator(cmd, args)
	}

	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = r.basedir
	c.Stdout = r.out
	c.Stderr = r.err
	c.Env = r.env
	return c.Run()
}
