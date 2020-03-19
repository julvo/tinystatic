package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v2"
)

func IsIndexFile(fileInfo os.FileInfo) bool {
	return strings.HasPrefix(fileInfo.Name(), "index.")
}

type Route struct {
	Children []Route
	FilePath string
	Href     string
	Meta     map[string]interface{}
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
			if IsIndexFile(child) {
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
		route.Href = relPath
		route.FilePath = fullPath
	}

	if route.FilePath != "" {
		fileContent, err := ioutil.ReadFile(route.FilePath)
		if err != nil {
			return route, err
		}
		if strings.HasPrefix(string(fileContent), "---") {
			err = yaml.Unmarshal(fileContent, &route.Meta)
			if err != nil {
				return route, err
			}
		}
	}

	fmt.Println(route.Href)
	fmt.Println(route.FilePath)
	fmt.Println("---")
	return route, nil
}

func (p *Route) AllChildren() []*Route {
	allChildren := []*Route{}
	for _, c := range p.Children {
		allChildren = append(append(allChildren, &c), c.AllChildren()...)
	}
	return allChildren
}
func (p *Route) AllRoutes() []*Route {
	return append([]*Route{p}, p.AllChildren()...)
}

func main() {
	var outputDir string
	flag.StringVar(&outputDir, "output", "./output", "The directory to write the generated outputs to")
	flag.Parse()

	root, err := LoadRoutes("/", "./routes")
	if err != nil {
		fmt.Println(err)
	}

	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			html.WithHardWraps(),
		),
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
			),
		),
	)

	for _, r := range root.AllRoutes() {
		if r.FilePath == "" {
			continue
		}

		switch filepath.Ext(r.FilePath) {
		case ".html", ".htm":
			os.MkdirAll(filepath.Join(outputDir, filepath.Dir(r.Href)), os.ModePerm)
			srcFile, err := os.Open(r.FilePath)
			if err != nil {
				log.Fatalln(err)
			}
			defer srcFile.Close()

			dstFile, err := os.Create(filepath.Join(outputDir, r.Href, "index.html"))
			if err != nil {
				log.Fatalln(err)
			}
			defer dstFile.Close()
			io.Copy(dstFile, srcFile)
		case ".md":
			dir := filepath.Join(outputDir, r.Href)
			os.MkdirAll(dir, os.ModePerm)
			srcFile, err := os.Open(r.FilePath)
			if err != nil {
				log.Fatalln(err)
			}
			defer srcFile.Close()

			var buf bytes.Buffer
			context := parser.NewContext()
			source, err := ioutil.ReadFile(r.FilePath)
			if err != nil {
				fmt.Println(err)
			}

			if err := markdown.Convert([]byte(source), &buf, parser.WithContext(context)); err != nil {
				panic(err)
			}

			tmplCtx := meta.Get(context)
			if tmplCtx == nil {
				tmplCtx = map[string]interface{}{}
			}

			sourceStr := string(source)
			if strings.HasPrefix(sourceStr, "---") {
				source = []byte(strings.SplitN(sourceStr, "---", 3)[2])
			}

			tmplCtx["body"] = template.HTML(buf.String())

			tmpl := template.Must(template.ParseFiles("template.html"))

			outFile, err := os.Create(filepath.Join(dir, "index.html"))
			if err != nil {
				fmt.Println("Could not open output file", err)
			}
			defer outFile.Close()

			err = tmpl.Execute(outFile, tmplCtx)
			if err != nil {
				fmt.Println(err)
			}
		default:
			os.MkdirAll(filepath.Join(outputDir, filepath.Dir(r.Href)), os.ModePerm)
			srcFile, err := os.Open(r.FilePath)
			if err != nil {
				log.Fatalln(err)
			}
			defer srcFile.Close()

			dstFile, err := os.Create(filepath.Join(outputDir, r.Href))
			if err != nil {
				log.Fatalln(err)
			}
			defer dstFile.Close()
			io.Copy(dstFile, srcFile)
		}

	}

	return
}
