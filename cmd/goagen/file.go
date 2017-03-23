package main

import (
	"text/template"

	"goa.design/goa.v2/codegen"
)

type (
	// mainFile is the codgen file for a given service.
	mainFile struct {
		// Generators contains the names of the generators to invoke.
		Generators []string
		// DesignPath is the Go import path of the design package
		DesignPath string
	}
)

// mainTmpl is the template used to render the body of the main file.
var mainTmpl = template.Must(template.New("main").Parse(mainT))

// Main returns the main file for the given service.
func Main(commands []string, designPath string) codegen.File {
	gens := make([]string, len(commands))
	for i, c := range commands {
		switch c {
		case "server":
			gens[i] = "Server"
		case "client":
			gens[i] = "Client"
		case "openapi":
			gens[i] = "OpenAPI"
		default:
			panic("unknown command " + c) // bug
		}
	}
	return &mainFile{Generators: gens, DesignPath: designPath}
}

// Sections returns the main file sections.
func (m *mainFile) Sections(genPkg string) []*codegen.Section {
	var (
		header, body *codegen.Section
	)
	{
		header = codegen.Header("Generator main", "main",
			[]*codegen.ImportSpec{
				{Path: "flag"},
				{Path: "fmt"},
				{Path: "os"},
				{Path: "sort"},
				{Path: "strings"},
				{Path: "goa.design/goa.v2/codegen"},
				{Path: "goa.design/goa.v2/codegen/generators"},
				{Path: "goa.design/goa.v2/eval"},
				{Path: "goa.design/goa.v2/pkg"},
				{Path: m.DesignPath, Name: "_"},
			})
		body = &codegen.Section{
			Template: mainTmpl,
			Data:     m.Generators,
		}
	}

	return []*codegen.Section{header, body}
}

// OutputPath is the path to the generated main file relative to the output
// directory.
func (m *mainFile) OutputPath(_ map[string]struct{}) string {
	return "main.go"
}

// mainT is the template for the generator main.
const mainT = `func main() {
	var (
		out     = flag.String("output", "", "")
		version = flag.String("version", "", "")
	)
	{
		flag.Parse()
		if *out == "" {
			fail("missing output flag")
		}
		if *version == "" {
			fail("missing version flag")
		}
	}

	if *version != pkg.Version() {
		fail("goa DSL was run with goa version %s but compiled generator is running %s\n", *version, pkg.Version())
	}
        if err := eval.Context.Errors; err != nil {
		fail(err.Error())
	}
	if err := eval.RunDSL(); err != nil {
		fail(err.Error())
	}

	var roots []eval.Root
	{
		rs, err := eval.Context.Roots()
		if err != nil {
			fail(err.Error())
		}
		roots = rs
	}

	var files []codegen.File
	{
{{- range . }}
		fs, err := generator.{{ . }}(roots...)
		if err != nil {
			fail(err.Error())
		}
		files = append(files, fs...)
{{ end }}	}

	var w *codegen.Writer
	{
		w = codegen.NewWriter(*out)
	}

	var outputs []string
	{
		outputs = make([]string, len(files))
		for i, file := range files {
			f, err := w.Write(file)
			if err != nil {
				fail(err.Error())
			}
			outputs[i] = f
		}
	}

	sort.Strings(outputs)
	fmt.Println(strings.Join(outputs, "\n"))
}

func fail(msg string, vals ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, vals...)
	os.Exit(1)
}
`
