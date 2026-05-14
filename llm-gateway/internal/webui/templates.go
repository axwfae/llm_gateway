package webui

import (
	"embed"
	"html/template"
	"log"
)

type PageData struct {
	Title   string
	Content template.HTML
}

//go:embed pages/*.html
var pagesFS embed.FS

const (
	LayoutFile    = "layout.html"
	IndexFile     = "index.html"
	ServersFile   = "servers.html"
	ModelsFile    = "models.html"
	APIKeysFile   = "apikeys.html"
	LocalModelsFile = "localmodels.html"
	SettingsFile  = "settings.html"
	TestResultsFile = "testresults.html"
)

func LoadTemplate() *template.Template {
	layoutData, err := pagesFS.ReadFile("pages/layout.html")
	if err != nil {
		log.Fatalf("Failed to read layout template: %v", err)
	}
	tmpl, err := template.New("main").Parse(string(layoutData))
	if err != nil {
		log.Fatalf("Failed to parse layout template: %v", err)
	}
	return tmpl
}

func LoadPage(filename string) template.HTML {
	data, err := pagesFS.ReadFile("pages/" + filename)
	if err != nil {
		return template.HTML("<p>Error loading page: " + filename + "</p>")
	}
	return template.HTML(string(data))
}
