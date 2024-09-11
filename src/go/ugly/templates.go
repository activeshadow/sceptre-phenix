package main

import (
	"html/template"
	"io"
	"path/filepath"
)

type Params struct {
	BasePath    string
	Experiments []string
	Experiment  string
	Topology    string
	Scenario    string
	VMs         []VM
	VMFilter    string
}

type VM struct {
	Name string
	DNB  bool
}

/*
//go:embed templates/*
var files embed.FS
*/

var (
	tmplPath = "/opt/phenix/ugly/templates"
	/*
		indexTmpl       *template.Template
		experimentsTmpl *template.Template
		experimentTmpl  *template.Template
	*/
)

/*
func init() {
	index = parse("index.html")
	experiments = parse("experiments.html")
	experiment = parse("experiment.html")
}
*/

func NewParams(exp string) Params {
	return Params{BasePath: basePath, Experiment: exp}
}

func parse(file string) *template.Template {
	return template.Must(
		// template.New("layout.html").ParseFS(files, "layout.html", file),
		template.New("layout.html").ParseFiles(filepath.Join(tmplPath, "layout.html"), filepath.Join(tmplPath, file)),
	)
}

func Index(w io.Writer, p Params) error {
	tmpl := parse("index.html")
	return tmpl.Execute(w, p)
}
