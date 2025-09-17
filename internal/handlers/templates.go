package handlers

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type TemplateCache map[string]*template.Template

type Renderer struct {
	cache TemplateCache
	log   *logrus.Logger
}

func NewRenderer() (*Renderer, error) {
	return NewRendererWithLogger(logrus.New())
}

func NewRendererWithLogger(logger *logrus.Logger) (*Renderer, error) {

	funcs := template.FuncMap{
		"currentDateTime": func() string {
			// Default to local timezone - this will be improved when we have user context
			return time.Now().Format("2006-01-02T15:04")
		},
		"currentDateTimeInTZ": func(tzName string) string {
			loc, err := time.LoadLocation(tzName)
			if err != nil {
				loc = time.Local // fallback
			}
			return time.Now().In(loc).Format("2006-01-02T15:04")
		},
		"humanDate": func(t time.Time) string {
			return t.Format("Jan 1, 2006 at 3:04pm")
		},
		"ISODate": func(t time.Time) string {
			return t.Format("2006-01-02T15:04:05")
		},
		"toJSON": func(data any) string {
			jd, _ := json.MarshalIndent(data, "", "  ")
			return string(jd)
		},
	}

	base, err := template.New("base").Funcs(funcs).ParseFiles("templates/layout.gohtml")
	if err != nil {
		logger.WithFields(logrus.Fields{
			"component": "renderer",
			"action":    "init",
			"file":      "templates/layout.gohtml",
			"error":     err.Error(),
		}).Fatal("Failed to parse base template file")
		return nil, err
	}

	cache := make(TemplateCache)

	// Partials: standalone templates that still have the same FuncMap
	partials, err := filepath.Glob("templates/partials/*.gohtml")
	if err != nil {
		return nil, err
	}
	for _, p := range partials {
		key := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		t := template.New(key).Funcs(funcs)
		if _, err := t.ParseFiles(p); err != nil {
			return nil, err
		}
		cache[key] = t
	}
	// Full pages
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

	logger.WithFields(logrus.Fields{
		"component":      "renderer",
		"action":         "init",
		"template_count": len(cache),
	}).Info("Template renderer initialized successfully")

	return &Renderer{cache: cache, log: logger}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := r.cache[name]
	if !ok {
		r.log.WithFields(logrus.Fields{
			"component": "renderer",
			"action":    "render",
			"template":  name,
		}).Error("Template does not exist in cache")
		http.Error(w, "template not found: "+name, http.StatusNotFound)
		return
	}

	r.log.WithFields(logrus.Fields{
		"component": "renderer",
		"action":    "render",
		"template":  name,
	}).Debug("Rendering template")

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		r.log.WithFields(logrus.Fields{
			"component": "renderer",
			"action":    "render",
			"template":  name,
			"error":     err.Error(),
		}).Error("Template execution failed")
		http.Error(w, "template execution error", http.StatusInternalServerError)
		return
	}

	if _, err := buf.WriteTo(w); err != nil {
		r.log.WithFields(logrus.Fields{
			"component": "renderer",
			"action":    "render",
			"template":  name,
			"error":     err.Error(),
		}).Error("Failed to write template response")
		return
	}

	r.log.WithFields(logrus.Fields{
		"component": "renderer",
		"action":    "render",
		"template":  name,
	}).Debug("Template rendered successfully")
}

func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data any) {
	tmpl, ok := r.cache[name]
	if !ok {
		r.log.WithFields(logrus.Fields{
			"component": "renderer",
			"action":    "render_partial",
			"template":  name,
		}).Error("Partial template does not exist in cache")
		http.Error(w, "partial not found: "+name, http.StatusNotFound)
		return
	}

	r.log.WithFields(logrus.Fields{
		"component": "renderer",
		"action":    "render_partial",
		"template":  name,
	}).Debug("Rendering partial template")

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		r.log.WithFields(logrus.Fields{
			"component": "renderer",
			"action":    "render_partial",
			"template":  name,
			"error":     err.Error(),
		}).Error("Partial template execution failed")
		http.Error(w, "partial template execution error", http.StatusInternalServerError)
		return
	}

	if _, err := buf.WriteTo(w); err != nil {
		r.log.WithFields(logrus.Fields{
			"component": "renderer",
			"action":    "render_partial",
			"template":  name,
			"error":     err.Error(),
		}).Error("Failed to write partial template response")
		return
	}

	r.log.WithFields(logrus.Fields{
		"component": "renderer",
		"action":    "render_partial",
		"template":  name,
	}).Debug("Partial template rendered successfully")
}
