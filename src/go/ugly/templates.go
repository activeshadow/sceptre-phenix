package main

import (
	"html/template"
	"io"
	"path/filepath"
	"strings"
)

type Params struct {
	BasePath    string
	Experiments []string
	Experiment  string
	Topology    string
	Scenario    string
	VMs         []VM
	VMFilter    string

	Running bool
	Modal   string
}

func (this Params) ExperimentRunning() string {
	if this.Running {
		return "you bet!"
	}

	return "sadly, no..."
}

func (this Params) ModalIsActive() string {
	if this.Modal != "" {
		return "is-active"
	}

	return ""
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
	tmplPath string
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

// TODO: refactor these at some point

type bannerConfig struct {
	BannerLines     []string `mapstructure:"banner"`
	BackgroundColor string   `mapstructure:"backgroundColor"`
	TextColor       string   `mapstructure:"textColor"`

	// Use type interface{} here so it can either be a simple string or a
	// template.HTML string (safe HTML).
	Banner interface{} `mapstructure:"-"`
}

type vncConfig struct {
	BasePath string
	Token    string
	ExpName  string
	VMName   string

	TopBanner    bannerConfig `mapstructure:"topBanner"`
	BottomBanner bannerConfig `mapstructure:"bottomBanner"`
}

func newVNCBannerConfig(exp, vm string) *vncConfig {
	return &vncConfig{
		BasePath: basePath,
		ExpName:  exp,
		VMName:   vm,
		TopBanner: bannerConfig{
			BackgroundColor: "white",
			TextColor:       "black",
		},
		BottomBanner: bannerConfig{
			BackgroundColor: "white",
			TextColor:       "black",
		},
	}
}

func (this *vncConfig) finalize(banner ...string) {
	if len(banner) > 0 {
		this.TopBanner.Banner = template.HTML(strings.Join(banner, "<br/>"))
		this.BottomBanner.Banner = template.HTML(strings.Join(banner, "<br/>"))
		return
	}

	if len(this.TopBanner.BannerLines) > 0 {
		this.TopBanner.Banner = template.HTML(strings.Join(this.TopBanner.BannerLines, "<br/>"))
	}

	if len(this.BottomBanner.BannerLines) > 0 {
		this.BottomBanner.Banner = template.HTML(strings.Join(this.BottomBanner.BannerLines, "<br/>"))
	}
}
