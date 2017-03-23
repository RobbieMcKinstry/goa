package codegen

import (
	"bytes"
	"fmt"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

type (
	// Writer encapsulates the state required to generate multiple files
	// in the context of a single goagen invocation.
	Writer struct {
		// Dir is the output directory.
		Dir string
		// Files list the relative generated file paths
		Files map[string]struct{}
	}

	// A File contains the logic to generate a complete file.
	File interface {
		// Sections is the list of file sections. genPkg is the Import
		// path to the gen package.
		Sections(genPkg string) []*Section
		// OutputPath returns the relative path to the output file.
		// The value must not be a key of reserved.
		OutputPath(reserved map[string]struct{}) string
	}

	// A Section consists of a template and accompanying render data.
	Section struct {
		// Template used to render section text.
		Template *template.Template
		// Data used as input of template.
		Data interface{}
	}

	// ImportSpec defines a generated import statement.
	ImportSpec struct {
		// Name of imported package if needed.
		Name string
		// Go import path of package.
		Path string
	}
)

// NewWriter initializes a writer that writes files in the given directory.
func NewWriter(dir string) *Writer {
	return &Writer{
		Dir:   dir,
		Files: make(map[string]struct{}),
	}
}

// Write generates the file produced by the given file writer.
// It returns the name of the file that ended up being written as it is different
// from the file output if it had to be changed to avoid overriding a file.
func (w *Writer) Write(file File) (string, error) {
	path := filepath.Join(w.Dir, file.OutputPath(w.Files))

	var wr io.Writer
	{
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return "", err
		}
		defer f.Close()
		wr = f
	}

	var genPkg string
	{
		pkg, err := build.ImportDir(w.Dir, build.FindOnly)
		if err != nil {
			return "", err
		}
		genPkg = pkg.ImportPath
	}

	for _, s := range file.Sections(genPkg) {
		if err := s.Write(wr); err != nil {
			return "", err
		}
	}
	if filepath.Ext(path) == ".go" {
		if err := formatCode(path); err != nil {
			return "", err
		}
	}
	w.Files[path] = struct{}{}

	return path, nil
}

// Write writes the section to the given writer.
func (s *Section) Write(w io.Writer) error {
	return s.Template.Execute(w, s.Data)
}

// Code returns the Go import statement for the ImportSpec.
func (s *ImportSpec) Code() string {
	if len(s.Name) > 0 {
		return fmt.Sprintf(`%s "%s"`, s.Name, s.Path)
	}
	return fmt.Sprintf(`"%s"`, s.Path)
}

// formatCode formats the given Go source file. It returns an error if the code
// cannot be parsed successfully.
func formatCode(path string) error {
	// Parse file into AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	content, _ := ioutil.ReadFile(path)
	if err != nil {
		var buf bytes.Buffer
		scanner.PrintError(&buf, err)
		return fmt.Errorf("%s\n========\nContent:\n%s", buf.String(), content)
	}

	// Clean unused imports
	imps := astutil.Imports(fset, file)
	for _, group := range imps {
		for _, imp := range group {
			path := strings.Trim(imp.Path.Value, `"`)
			if !astutil.UsesImport(file, path) {
				if imp.Name != nil {
					astutil.DeleteNamedImport(fset, file, imp.Name.Name, path)
				} else {
					astutil.DeleteImport(fset, file, path)
				}
			}
		}
	}

	// Format using goimports
	opts := imports.Options{Comments: true, TabIndent: true, TabWidth: 8, FormatOnly: true}
	// Due to a bug in imports.Process we need to load the file and pass the
	// bytes as second argument, see https://github.com/golang/go/issues/19676
	b, err := imports.Process(path, content, &opts)
	if err != nil {
		return err
	}
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer w.Close()
	w.Write(b)

	return nil
}
