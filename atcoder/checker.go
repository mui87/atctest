package atcoder

import (
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/mui87/atctest/commander"
)

type Checker struct {
	commander commander.Commander
	outStream io.Writer
	errStream io.Writer
}

func NewChecker(outStream, errStream io.Writer) *Checker {
	return &Checker{
		commander: commander.NewExternal(),
		outStream: outStream,
		errStream: errStream,
	}
}

func (c *Checker) Check(command string, samples []Sample) bool {
	successAll := true
	for i, sample := range samples {
		success, actual, err := c.checkOne(command, sample)
		_, _ = fmt.Fprintf(c.outStream, "sample %d: ", i+1)
		if err != nil {
			successAll = false

			_, _ = color.New(color.FgRed).Fprintln(c.outStream, "ERROR")
			_, _ = fmt.Fprintln(c.outStream, err.Error())
		} else if success {
			_, _ = color.New(color.FgGreen).Fprintln(c.outStream, "SUCCESS")
		} else {
			successAll = false

			_, _ = color.New(color.FgRed).Fprintln(c.outStream, "FAILURE")
			_, _ = fmt.Fprintln(c.outStream, "input:")
			_, _ = fmt.Fprint(c.outStream, sample.Input)
			_, _ = fmt.Fprintln(c.outStream, "expected output:")
			_, _ = fmt.Fprint(c.outStream, sample.Output)
			_, _ = fmt.Fprintln(c.outStream, "actual output:")
			_, _ = fmt.Fprint(c.outStream, actual)
		}
	}

	return successAll
}

func (c *Checker) checkOne(command string, sample Sample) (bool, string, error) {
	actualOutput, err := c.commander.Run(command, sample.Input)
	if err != nil {
		return false, "", err
	}
	success := actualOutput == sample.Output

	return success, actualOutput, nil
}
