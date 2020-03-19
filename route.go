package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v2"
)

type Route struct {
	Children []Route
	FilePath string
	Href     string
	Meta     map[string]interface{}
}

func (r *Route) Generate(outputDir string) error {
	if r.Href == "" {
		return nil
	}

	src, err := ioutil.ReadFile(r.FilePath)
	if err != nil {
		return err
	}

	switch ext := strings.ToLower(filepath.Ext(r.FilePath)); ext {
	case ".md", ".markdown":
		html, err := markdownToHTML(src)
		if err != nil {
			return err
		}

		if _, useTmpl := r.Meta["template"]; useTmpl {
			html = []byte(`{{define "body"}}` + string(html) + `{{end}}`)
		}

		src = html
		fallthrough

	case ".html", ".htm":
		if err := os.MkdirAll(filepath.Join(outputDir, r.Href), os.ModePerm); err != nil {
			return err
		}

		dstPath := filepath.Join(outputDir, r.Href, "index.html")
		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		tmplName, useTmpl := r.Meta["template"]
		if !useTmpl {
			if _, err := dstFile.Write(src); err != nil {
				return err
			}
		} else {
			tmplPath := filepath.Join(templateDir, fmt.Sprint(tmplName))
			partials, err := filepath.Glob(filepath.Join(partialDir, "*.html"))
			if err != nil {
				return err
			}

			tmpl, err := template.ParseFiles(append([]string{tmplPath}, partials...)...)
			if err != nil {
				return err
			}

			if strings.HasPrefix(string(src), "---") {
				src = []byte(strings.SplitN(string(src), "---", 3)[2])
			}

			tmpl, err = tmpl.Parse(string(src))
			if err != nil {
				return err
			}

			// TODO more context, e.g. other routes
			if err := tmpl.Execute(dstFile, r.Meta); err != nil {
				return err
			}
		}

	default:
		if err := os.MkdirAll(filepath.Join(outputDir, filepath.Dir(r.Href)), os.ModePerm); err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(outputDir, r.Href), src, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func (r Route) AllChildren() []Route {
	allChildren := []Route{}
	for _, c := range r.Children {
		allChildren = append(append(allChildren, c), c.AllChildren()...)
	}
	return allChildren
}
func (r Route) AllRoutes() []Route {
	return append([]Route{r}, r.AllChildren()...)
}

func LoadRoutes(relPath, baseDir string) (Route, error) {
	fullPath := filepath.Join(baseDir, relPath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return Route{}, err
	}

	route := Route{}
	if fileInfo.IsDir() {
		fileInfos, err := ioutil.ReadDir(fullPath)
		if err != nil {
			return Route{}, err
		}

		for _, child := range fileInfos {
			if isIndexFile(child) {
				route.Href = relPath
				route.FilePath = filepath.Join(fullPath, child.Name())
			} else {
				childRoute, err := LoadRoutes(filepath.Join(relPath, child.Name()), baseDir)
				if err != nil {
					return route, err
				}
				route.Children = append(route.Children, childRoute)
			}
		}

	} else {
		route.Href = filePathToHref(relPath)
		route.FilePath = fullPath
	}

	if route.FilePath != "" {
		fileContent, err := ioutil.ReadFile(route.FilePath)
		if err != nil {
			return route, err
		}
		if strings.HasPrefix(string(fileContent), "---") {
			if err := yaml.Unmarshal(fileContent, &route.Meta); err != nil {
				return route, err
			}
		}
	}

	return route, nil
}

func markdownToHTML(src []byte) ([]byte, error) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			html.WithHardWraps()),
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"))))

	var buf bytes.Buffer
	if err := markdown.Convert(src, &buf); err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func filePathToHref(fpath string) string {
	switch ext := strings.ToLower(filepath.Ext(fpath)); ext {
	case ".html", ".htm", ".md", ".markdown":
		return strings.TrimSuffix(fpath, ext)
	}
	return fpath
}

func isIndexFile(fileInfo os.FileInfo) bool {
	return strings.HasPrefix(fileInfo.Name(), "index.")
}
