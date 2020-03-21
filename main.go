package main

import (
	"flag"
	"log"
	"os"
)

var (
	partialDir  string
	templateDir string
)

func main() {
	var outputDir string
	var routeDir string
	var clean bool
	flag.StringVar(&outputDir, "output", "./output", "The directory to write the generated outputs to")
	flag.StringVar(&routeDir, "routes", "./routes", "The directory from which to read the routes")
	flag.StringVar(&partialDir, "partials", "./partials", "The directory from which to read the partials")
	flag.StringVar(&templateDir, "templates", "./templates", "The directory from which to read the templates")
	flag.BoolVar(&clean, "clean", false, "Whether to delete the output directory before regenerating")
	flag.Parse()

	if clean {
		log.Println("Removing previous output from", outputDir)
		if err := os.RemoveAll(outputDir); err != nil {
			log.Fatalln(err)
		}
	}

	log.Println("Loading routes from", routeDir)
	rootRoute, err := LoadRoutes("/", routeDir)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Writing output to", outputDir)
	allRoutes := rootRoute.AllRoutes()
	for _, r := range allRoutes {
		if r.Href != "" {
			log.Println("∟", r.FilePath, "->", r.Href)
		}
		if err := r.Generate(outputDir, allRoutes); err != nil {
			log.Fatalln(err)
		}
	}
	log.Println("Finished")
}
