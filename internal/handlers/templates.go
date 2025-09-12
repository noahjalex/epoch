package handlers

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type TemplateCache map[string]*template.Template

type Renderer struct {
	cache TemplateCache
}

func NewRenderer() (*Renderer, error) {

	funcs := template.FuncMap{
		"time": func() string {
			return time.Now().Format("2006-01-02 15:04:05")
		},
		"toJSON": func(data any) string {
			jd, _ := json.MarshalIndent(data, "", "  ")
			return string(jd)
		},
	}

	base, err := template.New("base").Funcs(funcs).ParseFiles("templates/layout.gohtml")
	if err != nil {
		log.Fatalf("parse base err %v", err)
	}

	if partials, _ := filepath.Glob("templates/partials/*.gohtml"); len(partials) > 0 {
		if _, err := base.ParseFiles(partials...); err != nil {
			return nil, err
		}
	}

	cache := make(TemplateCache)

	pageFiles, err := filepath.Glob("templates/pages/*.gohtml")
	if err != nil {
		return nil, err
	}

	for _, f := range pageFiles {
		// Skip the layout
		if filepath.Base(f) == "layout.gohtml" {
			continue
		}

		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}

		// Parse each page into base
		if _, err := clone.ParseFiles(f); err != nil {
			return nil, err
		}

		key := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		cache[key] = clone
	}

	return &Renderer{cache}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := r.cache[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusNotFound)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		http.Error(w, "template execution error", http.StatusInternalServerError)
		log.Printf("execute %q: %v", name, err)
		return
	}

	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("writing response for %q: %v", name, err)
		return
	}
}
