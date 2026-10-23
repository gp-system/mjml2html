// Command mjml2html compiles MJML from a file or standard input.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"mjml2html"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	output := flag.String("o", "", "output file (default: stdout)")
	includes := flag.Bool("includes", false, "enable includes relative to the input file (or current directory for stdin)")
	flag.Parse()
	if flag.NArg() > 1 {
		return fmt.Errorf("usage: mjml2html [-o output.html] [-includes] [input.mjml]")
	}
	var input io.Reader = os.Stdin
	dir := "."
	if flag.NArg() == 1 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
		dir = filepath.Dir(flag.Arg(0))
	}
	source, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	options := mjml2html.Options{}
	if *includes {
		options.IncludeFS = os.DirFS(dir)
	}
	result, err := mjml2html.Render(string(source), options)
	if err != nil {
		return err
	}
	if *output != "" {
		return os.WriteFile(*output, []byte(result), 0644)
	}
	_, err = io.WriteString(os.Stdout, result)
	return err
}
