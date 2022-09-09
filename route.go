package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/sprig/v3"
	"github.com/dop251/goja"
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

	rawHref string
	rawMeta map[string]interface{}
}

var funcMap map[string]interface{}

func init() {
	funcMap = sprig.FuncMap()

	for k, v := range map[string]interface{}{
		"sortAsc":         sortAsc,
		"sortDesc":        sortDesc,
		"limit":           limit,
		"offset":          offset,
		"filter":          filter,
		"filterHref":      filterHref,
		"filterFileName":  filterFileName,
		"filterFilePath":  filterFilePath,
		"fn":              fn,
		"toUnescapedJson": toUnescapedJson,
	} {
		funcMap[k] = v
	}
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
		tmpl = tmpl.Funcs(funcMap)

		if len(tmplFiles) > 0 {
			tmpl, err = tmpl.ParseFiles(tmplFiles...)
			if err != nil {
				return err
			}
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
				route.rawHref = relPath
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
		route.rawHref = filePathToHref(relPath)
		route.FilePath = fullPath
	}

	route.Href = route.rawHref

	if route.FilePath != "" {
		fileContent, err := ioutil.ReadFile(route.FilePath)
		if err != nil {
			return route, err
		}
		if strings.HasPrefix(string(fileContent), "---") {
			if err := yaml.Unmarshal(fileContent, &route.rawMeta); err != nil {
				return route, err
			}

			route.Meta = make(map[string]interface{}, len(route.rawMeta))
			for k, v := range route.rawMeta {
				route.Meta[k] = v
			}
		}
	}

	return route, nil
}

func EvalMetaExpressions(routes []Route) error {
	for i, r := range routes {
		for name, val := range r.rawMeta {
			if str, ok := val.(string); ok && strings.HasPrefix(strings.TrimSpace(str), "{{") {
				tmplStr := strings.Replace(str, "}}", " | toUnescapedJson }}", -1)
				tmpl := template.New(r.FilePath)
				tmpl = tmpl.Funcs(funcMap)
				tmpl, err := tmpl.Parse(tmplStr)
				if err != nil {
					return err
				}

				tmplCtx := map[string]interface{}{}
				for k, v := range r.rawMeta {
					tmplCtx[k] = v
				}
				tmplCtx["Route"] = r
				tmplCtx["Routes"] = routes

				var resultBytes bytes.Buffer
				if err := tmpl.Execute(&resultBytes, tmplCtx); err != nil {
					return err
				}

				var result interface{}
				if err := json.Unmarshal(resultBytes.Bytes(), &result); err != nil {
					return err
				}

				routes[i].Meta[name] = result
			} else {
				routes[i].Meta[name] = val
			}
		}
	}

	return nil
}

func ExpandRoutes(route *Route) error {
	for {

		oldRoutes := route.AllRoutes()
		oldHrefs := make([]string, len(oldRoutes))
		for _, r := range oldRoutes {
			oldHrefs = append(oldHrefs, r.Href)
		}

		if err := ExpandDynamicRoutes(route); err != nil {
			return err
		}

		newRoutes := route.AllRoutes()
		newHrefs := make([]string, len(newRoutes))
		for _, r := range newRoutes {
			newHrefs = append(newHrefs, r.Href)
		}

		if isEqualStringSet(oldHrefs, newHrefs) {
			return nil
		}
	}
}

func isEqualStringSet(a, b []string) bool {
	aMap := make(map[string]interface{}, len(a))
	for _, aElem := range a {
		aMap[aElem] = nil
	}
	bMap := make(map[string]interface{}, len(b))
	for _, bElem := range b {
		bMap[bElem] = nil
		if _, ok := aMap[bElem]; !ok {
			return false
		}
	}
	if len(aMap) != len(bMap) {
		return false
	}

	return true
}

func ExpandDynamicRoutes(route *Route) error {
	regex := *regexp.MustCompile(`\[([^\]]+)\]`)

	if err := EvalMetaExpressions(route.AllRoutes()); err != nil {
		return err
	}

	for _, r := range route.AllRoutes() {
		matches := regex.FindAllStringSubmatch(r.FilePath, -1)
		variables := make(map[string][]interface{}, len(matches))
		for i := range matches {
			name := strings.TrimSpace(matches[i][1])
			value := r.Meta[name]

			switch reflect.TypeOf(value).Kind() {
			case reflect.Slice, reflect.Array:
				variables[name] = value.([]interface{})
			default:
				variables[name] = []interface{}{value}
			}
		}

		if len(variables) < 1 {
			continue
		}

		variableNames := make([]string, len(variables))
		variableValues := make([][]interface{}, len(variables))
		varIdx := 0
		for varName, varValue := range variables {
			variableNames[varIdx] = varName
			variableValues[varIdx] = varValue
			varIdx += 1
		}

		permutations := eachPermutation(variableValues...)

		newRoutes := make([]Route, len(permutations))

		for permIdx, permutation := range permutations {
			href := r.rawHref
			newRoutes[permIdx] = r

			// TODO improve deep copying
			newRoutes[permIdx].Meta = make(map[string]interface{}, len(r.Meta))
			for k, v := range r.Meta {
				newRoutes[permIdx].Meta[k] = v
			}

			for varIdx, varName := range variableNames {
				varValue := permutation[varIdx]
				regex := *regexp.MustCompile(`\[\s*` + varName + `\s*\]`)
				href = regex.ReplaceAllString(href, fmt.Sprint(varValue))

				newRoutes[permIdx].Meta[varName] = varValue
			}
			newRoutes[permIdx].Href = href
		}

		replaceAllRoutesForFile(route, r.FilePath, newRoutes)
	}

	return nil
}

func replaceAllRoutesForFile(route *Route, filePath string, replaceWith []Route) {
	oldChildren := route.Children
	route.Children = make([]Route, 0, len(oldChildren))
	found := false
	for _, child := range oldChildren {
		if child.FilePath != filePath {
			route.Children = append(route.Children, child)
		} else {
			if !found {
				route.Children = append(route.Children, replaceWith...)
			}
			found = true
		}
	}

	if found {
		return
	}

	for i := range route.Children {
		replaceAllRoutesForFile(&route.Children[i], filePath, replaceWith)
	}
}

func eachPermutation(values ...[]interface{}) [][]interface{} {
	// [[a, b], [1, 2, 3]]
	// ->
	// [[a, 1], [a, 2], [a, 3], [b, 1], [b, 2], [b, 3]]

	length := 0
	for _, vals := range values {
		if length == 0 {
			length = 1
		}
		length *= len(vals)
	}

	permutations := make([][]interface{}, length)

	for i := range permutations {
		permutations[i] = make([]interface{}, len(values))
		acc := 1
		for j := range permutations[i] {
			permutations[i][j] = values[j][(acc*i*len(values[j])/length)%len(values[j])]
			acc *= len(values[j])
		}

	}

	return permutations
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

func fn(source string, args ...interface{}) interface{} {
	vm := goja.New()
	v, err := vm.RunString(source)
	if err != nil {
		panic(err)
	}

	f, ok := goja.AssertFunction(v)
	if !ok {
		panic("Not a function")
	}

	jsArgs := make([]goja.Value, len(args))

	for i := 0; i < len(args); i++ {
		jsArgs[i] = vm.ToValue(args[i])
	}

	res, err := f(goja.Undefined(), jsArgs...)

	if err != nil {
		panic(err)
	}

	return res.Export()
}

func toUnescapedJson(val interface{}) template.HTML {
	result, _ := json.Marshal(val)
	return template.HTML(string(result))
}
