// Command generate-template writes a correct (but simple) starter order book
// in the chosen language into an output directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	language := flag.String("language", "cpp", "cpp|rust|go|python")
	output := flag.String("output", "./contestant-starter", "output directory")
	flag.Parse()

	files, ok := templates[*language]
	if !ok {
		fmt.Fprintf(os.Stderr, "unsupported language %q (cpp|rust|go|python)\n", *language)
		os.Exit(1)
	}
	if err := os.MkdirAll(*output, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for name, content := range files {
		path := filepath.Join(*output, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("wrote", path)
	}
	fmt.Printf("starter (%s) generated in %s\n", *language, *output)
}
