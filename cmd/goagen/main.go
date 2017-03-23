package main

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"goa.design/goa.v2/codegen"
	"goa.design/goa.v2/pkg"

	"flag"
)

func main() {
	var (
		cmds   []string
		path   string
		offset int
	)
	{
		switch os.Args[1] {
		case "version":
			fmt.Println("goagen version " + pkg.Version())
			os.Exit(0)

		case "client", "server", "openapi":
			if len(os.Args) == 2 {
				usage()
			}
			cm := map[string]struct{}{os.Args[1]: struct{}{}}
			offset = 2
			for len(os.Args) > offset+1 &&
				(os.Args[offset] == "client" ||
					os.Args[offset] == "server" ||
					os.Args[offset] == "openapi") {
				cm[os.Args[offset]] = struct{}{}
				offset++
			}
			for cmd := range cm {
				cmds = append(cmds, cmd)
			}
			sort.Strings(cmds)

		default:
			cmds = []string{"client", "openapi", "server"}
			offset = 1
		}

		path = os.Args[offset]
	}

	var (
		output      = "."
		gens, debug bool
	)
	if len(os.Args) > offset+1 {
		var (
			fset     = flag.NewFlagSet("default", flag.ExitOnError)
			o        = fset.String("o", "", "output `directory`")
			out      = fset.String("output", output, "output `directory`")
			s        = fset.Bool("s", false, "Generate scaffold (does not override existing files)")
			scaffold = fset.Bool("scaffold", false, "Generate scaffold (does not override existing files)")
		)
		fset.BoolVar(&debug, "debug", false, "Print debug information")

		fset.Usage = usage
		fset.Parse(os.Args[offset+1:])

		output = *o
		if output == "" {
			output = *out
		}

		gens = *s
		if !gens {
			gens = *scaffold
		}
	}

	out, err := gen(cmds, path, output, gens, debug)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Print(out)
	return
}

// help with tests
var (
	usage = help
	gen   = generate
)

func generate(cmds []string, path, output string, gens, debug bool) (string, error) {
	if _, err := build.Import(path, ".", build.IgnoreVendor); err != nil {
		return "", err
	}

	gobin, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf(`failed to find a go compiler, looked in "%s"`, os.Getenv("PATH"))
	}

	// Write generator source in temporary directory
	wd := "."
	if cwd, err := os.Getwd(); err != nil {
		wd = cwd
	}
	tmpDir, err := ioutil.TempDir(wd, "goagen")
	if err != nil {
		return "", err
	}
	if !debug {
		defer os.RemoveAll(tmpDir)
	}

	w := codegen.NewWriter(tmpDir)
	if _, err = w.Write(Main(cmds, path)); err != nil {
		return "", err
	}

	// Compile generator
	out := "goagen"
	if runtime.GOOS == "windows" {
		out += ".exe"
	}

	c := exec.Cmd{Path: gobin, Args: []string{gobin, "build", "-o", out}, Dir: tmpDir}
	cout, err := c.CombinedOutput()
	if err != nil {
		if len(cout) > 0 {
			err = fmt.Errorf(string(cout))
		}
		return "", fmt.Errorf("failed to compile generator: %s", err)
	}

	// Run generator
	args := []string{"--version=" + pkg.Version(), "--output=" + output}
	cmd := exec.Command(filepath.Join(tmpDir, out), args...)
	cout, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s\n%s", err, string(cout))
	}

	return string(cout), nil
}

func help() {
	fmt.Fprint(os.Stderr, `goagen is the goa code generation tool.
Learn more about goa at https://goa.design.

The tool supports multiple subcommands that generate different outputs.
The only argument is the Go import path to the service design package.

If no command is specified then all commands are run.

The "-scaffold" flag tells goagen to also generate the scaffold for the server
and/or the client depending on which command is being executed. The scaffold is
code that contains placeholders and is generated once to help get started quickly.

Usage:

  goagen [server] [client] [openapi] PACKAGE [-out DIRECTORY] [-scaffold] [-debug]

  goagen version

Commands:
  server
        Generate service interfaces, endpoints and server transport code.

  client
        Generate endpoints and client transport code.

  openapi
        Generate OpenAPI specification (https://www.openapis.org/).

  version
        Print version information (exclusive with other flags and commands).

Args:
  PACKAGE
        Go import path to design package

Flags:
  -o, -output DIRECTORY
        output directory, defaults to the current working directory

  -s, -scaffold
        generate scaffold (does not override existing files)

  -debug
        Print debug information (mainly intended for goa developers)

Examples:

Bootstrap a new service:
  goagen goa.design/cellar/design -s

(Re)Generate the server code and OpenAPI spec only:
  goagen server openapi goa.design/cellar/design

`)
	os.Exit(1)
}
