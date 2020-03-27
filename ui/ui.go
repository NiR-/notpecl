package ui

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type UI interface {
	Prompt(question, defaultVal string) (string, error)
}

type interactive struct {
	in  io.Reader
	out io.Writer
}

func NewInteractiveUI(in io.Reader, out io.Writer) UI {
	return interactive{
		in:  in,
		out: out,
	}
}

func (ui interactive) Prompt(question, defaultVal string) (string, error) {
	qline := fmt.Sprintf("%s [%s]: ", question, defaultVal)
	_, err := ui.out.Write([]byte(qline))
	if err != nil {
		return "", err
	}

	r := bufio.NewReader(ui.in)
	val, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	val = strings.TrimSuffix(val, "\n")
	if val == "" {
		val = defaultVal
	}

	return val, nil
}

type nonInteractive struct{}

func NewNonInteractiveUI() UI {
	return nonInteractive{}
}

func (ui nonInteractive) Prompt(question, defaultValue string) (string, error) {
	return defaultValue, nil
}
