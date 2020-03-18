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

func GetIndexFile(dir string) (string, bool, error) {
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", false, err
	}

	for _, f := range fileInfos {
		if IsIndexFile(f) {
			return filepath.Join(dir, f.Name()), true, nil
		}
	}

	return "", false, nil
}

func LoadPages(dir string) (Page, error) {
	index, hasIndex, err := GetIndexFile(dir)
	if err != nil {
		return Page{}, err
	}

	children, err := LoadChildPages("", dir)
	if err != nil {
		return Page{}, err
	}
	page := Page{
		Href:     "/",
		FilePath: index,
		IsPage:   hasIndex,
		IsDir:    true,
		Children: children,
	}

	if !hasIndex {
		return page, nil
	}

	fileContent, err := ioutil.ReadFile(page.FilePath)
	if err != nil {
		return page, err
	}
	err = yaml.Unmarshal(fileContent, &page.Meta)
	if err != nil {
		return page, err
	}
	return page, nil
}

func LoadChildPages(relPath string, baseDir string) ([]Page, error) {
	pages := []Page{}

	dir := filepath.Join(baseDir, relPath)

	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return pages, err
	}

	for _, f := range fileInfos {
		var page Page
		href := filepath.Join(relPath, f.Name())
		fpath := filepath.Join(baseDir, href)

		if IsIndexFile(f) {
			continue
		}

		switch {
		case f.IsDir():
			children, err := LoadChildPages(href, baseDir)
			if err != nil {
				return pages, err
			}

			indexFile, hasIndex, err := GetIndexFile(fpath)
			if err != nil {
				return pages, err
			}

			page = Page{
				Href:     href,
				FilePath: indexFile,
				Children: children,
				IsDir:    true,
				IsPage:   hasIndex,
			}

		//	page = Page{
		//		Href:     relPath,
		//		FilePath: fpath,
		//		IsDir:    false,
		//	}
		//	fileContent, err := ioutil.ReadFile(fpath)
		//	if err != nil {
		//		return pages, err
		//	}
		//	err = yaml.Unmarshal(fileContent, &page.Meta)
		//	if err != nil {
		//		return pages, err
		//	}
		default:
			ext := strings.ToLower(filepath.Ext(href))
			if ext == ".html" || ext == ".htm" || ext == ".md" {
				page = Page{
					Href:     strings.TrimSuffix(href, filepath.Ext(href)),
					FilePath: fpath,
					IsPage:   true,
					IsDir:    false,
				}
			} else {
				page = Page{
					Href:     href,
					FilePath: fpath,
					IsPage:   false,
					IsDir:    false,
				}
			}
		}

		if page.IsPage {
			fileContent, err := ioutil.ReadFile(page.FilePath)
			if err != nil {
				return pages, err
			}
			err = yaml.Unmarshal(fileContent, &page.Meta)
			if err != nil {
				return pages, err
			}
		}
		fmt.Println("---")
		fmt.Println(page.Href)
		fmt.Println(page.FilePath)
		fmt.Println(page.IsDir)
		fmt.Println(page.Meta)

		pages = append(pages, page)
	}

	return pages, nil
}

type Page struct {
	IsPage   bool
	Children []Page
	Href     string
	FilePath string
	IsDir    bool
	Meta     map[string]interface{}
}

func (p *Page) AllChildren() []*Page {
	allChildren := []*Page{}
	for _, c := range p.Children {
		allChildren = append(append(allChildren, &c), c.AllChildren()...)
	}
	return allChildren
}
func (p *Page) AllPages() []*Page {
	return append([]*Page{p}, p.AllChildren()...)
}

func main() {
	var outputDir string
	flag.StringVar(&outputDir, "output", "./output", "The directory to write the generated outputs to")
	flag.Parse()

	root, err := LoadPages("./routes")
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

	for _, p := range root.AllPages() {
		fmt.Println("  " + p.Href + ": " + p.FilePath)

		if !p.IsDir && !p.IsPage {
			os.MkdirAll(filepath.Join(outputDir, filepath.Dir(p.Href)), os.ModePerm)

			srcFile, err := os.Open(p.FilePath)
			if err != nil {
				log.Fatalln(err)
			}
			defer srcFile.Close()

			dstFile, err := os.Create(filepath.Join(outputDir, p.Href))
			if err != nil {
				log.Fatalln(err)
			}
			defer dstFile.Close()
			io.Copy(dstFile, srcFile)
			continue
		}

		if !p.IsPage {
			continue
		}

		dir := filepath.Join(outputDir, p.Href)
		os.MkdirAll(dir, os.ModePerm)
		srcFile, err := os.Open(p.FilePath)
		if err != nil {
			log.Fatalln(err)
		}
		defer srcFile.Close()

		var buf bytes.Buffer
		context := parser.NewContext()
		source, err := ioutil.ReadFile(p.FilePath)
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
	}

	return
}
