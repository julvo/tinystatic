package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
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

func (r *Route) Generate(outputDir string, allRoutes []Route) error {
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
		partials, err := filepath.Glob(filepath.Join(partialDir, "*.html"))
		if err != nil {
			return err
		}

		var tmplFiles []string
		var tmplPath string
		if useTmpl {
			tmplPath = filepath.Join(templateDir, fmt.Sprint(tmplName))
			tmplFiles = append([]string{tmplPath}, partials...)
		} else {
			tmplPath = r.FilePath
			tmplFiles = partials
		}

		tmpl := template.New(filepath.Base(tmplPath))
		tmpl = tmpl.Funcs(map[string]interface{}{
			"sortAsc":        sortAsc,
			"sortDesc":       sortDesc,
			"limit":          limit,
			"offset":         offset,
			"filter":         filter,
			"filterHref":     filterHref,
			"filterFileName": filterFileName,
			"filterFilePath": filterFilePath,
		})

		tmpl, err = tmpl.ParseFiles(tmplFiles...)
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

		tmplCtx := map[string]interface{}{}
		for k, v := range r.Meta {
			tmplCtx[k] = v
		}
		tmplCtx["Route"] = r
		tmplCtx["Routes"] = allRoutes

		if err := tmpl.Execute(dstFile, tmplCtx); err != nil {
			return err
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

func sortAsc(sortBy string, routes []Route) []Route {
	sorted := make([]Route, len(routes))
	copy(sorted, routes)
	sort.Slice(sorted, func(i, j int) bool { return fmt.Sprint(routes[i].Meta[sortBy]) < fmt.Sprint(routes[j].Meta[sortBy]) })
	return sorted
}

func sortDesc(sortBy string, routes []Route) []Route {
	sorted := make([]Route, len(routes))
	copy(sorted, routes)
	sort.Slice(sorted, func(i, j int) bool { return fmt.Sprint(routes[i].Meta[sortBy]) > fmt.Sprint(routes[j].Meta[sortBy]) })
	return sorted
}

func limit(limit int, routes []Route) []Route {
	if limit >= len(routes) {
		return routes
	}
	return routes[:limit]
}

func offset(offset int, routes []Route) []Route {
	if offset >= len(routes) {
		return []Route{}
	}
	return routes[offset:]
}

func filter(metaKey, metaPattern string, routes []Route) []Route {
	filtered := []Route{}
	for _, r := range routes {
		match, err := filepath.Match(metaPattern, fmt.Sprint(r.Meta[metaKey]))
		if err != nil {
			fmt.Println("Warning in Filter: Could not match", metaPattern, "with", r.Meta[metaKey])
			continue
		}
		if match {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func filterHref(hrefPattern string, routes []Route) []Route {
	filtered := []Route{}
	for _, r := range routes {
		match, err := filepath.Match(hrefPattern, r.Href)
		if err != nil {
			fmt.Println("Warning in FilterHref: Could not match", hrefPattern, "with", r.Href)
			continue
		}
		if match {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

func filterFilePath(filePathPattern string, routes []Route) []Route {
	filtered := []Route{}
	for _, r := range routes {
		match, err := filepath.Match(filePathPattern, r.FilePath)
		if err != nil {
			fmt.Println("Warning in FilterFilePath: Could not match", filePathPattern, "with", r.FilePath)
			continue
		}
		if match {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

func filterFileName(fileNamePattern string, routes []Route) []Route {
	filtered := []Route{}
	for _, r := range routes {
		fname := filepath.Base(r.FilePath)
		match, err := filepath.Match(fileNamePattern, fname)
		if err != nil {
			fmt.Println("Warning in FilterFileName: Could not match", fileNamePattern, "with", fname)
			continue
		}
		if match {
			filtered = append(filtered, r)
		}
	}

	return filtered
}
