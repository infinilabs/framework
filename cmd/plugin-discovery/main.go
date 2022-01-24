package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

var usageText = `
Usage: plugin-discovery [flags]
  plugin-discovery is a tool that auto discovery module or plugins, generate a go file that include in main.go
Options:
`[1:]

var (
	pkg               string
	importPrefix               string
	outFile           string
	pluginDirs        stringSliceFlag
)

func init() {
	flag.Var(&pluginDirs, "dir", "Directory to search for plugins")
	flag.StringVar(&importPrefix, "import_prefix", "infini.sh/gateway/", "Prefix for generated package path")
	flag.StringVar(&pkg, "pkg", "config", "Package name for generated go file")
	flag.StringVar(&outFile, "out", "config/plugins.go", "Output filename")
	flag.Usage = usageFlag
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	if len(pluginDirs) == 0 {
		log.Fatal("Dir is required")
	}

	// Build import paths.
	var imports =map[string]util.KV{}
	for _, dir := range pluginDirs {

		libRegEx, e := regexp.Compile(".*.go$")
		if e != nil {
			log.Fatal(e)
		}

		e = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {

			if path==outFile{
				return nil
			}

			if err == nil && libRegEx.MatchString(info.Name()) {

				if strings.HasSuffix(info.Name(), "_test.go") {
					return nil
				}
				if hasInitMethod(filepath.Join(path)) {
					imports[filepath.ToSlash(
						filepath.Join(importPrefix, filepath.Dir(path)))] =util.KV{}

					return nil
				}
			}
			return nil
		})
		if e != nil {
			log.Fatal(e)
		}
	}

	importKeys:=util.GetMapKeys(imports)

	// Populate the template.
	var buf bytes.Buffer
	err := Template.Execute(&buf, Data{
		Package:   pkg,
		Imports:   importKeys,
	})
	if err != nil {
		log.Fatalf("Failed executing template: %v", err)
	}

	// Create the output directory.
	if err = os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Write the output file.
	if err = ioutil.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
		log.Fatalf("Failed writing output file: %v", err)
	}
}

func usageFlag() {
	fmt.Fprintf(os.Stderr, usageText)
	flag.PrintDefaults()
}

var Template = template.Must(template.New("normalizations").Funcs(map[string]interface{}{
	"trim": strings.TrimSpace,
}).Parse(
`/// GENERATED CODE BY PLUGIN DISCOVERY- DO NOT EDIT.

package {{ .Package }}

import (
{{- range $import := .Imports }}
	_ "{{ $import }}"
{{- end }}
)
`[1:]))

type Data struct {
	Package   string
	Imports   []string
}

//stringSliceFlag is a flag type that allows more than one value to be specified.
type stringSliceFlag []string

func (f *stringSliceFlag) String() string { return strings.Join(*f, ",") }

func (f *stringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// hasInitMethod returns true if the file contains 'func init()'.
func hasInitMethod(file string) bool {

	//fmt.Println("checking:",file)

	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("Failed to read from %v: %v", file, err)
	}
	defer f.Close()

	var initSignature = []byte("func init()")
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), initSignature) {
			return true
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed scanning %v: %v", file, err)
	}
	return false
}


// FindFiles return a list of file matching the given glob patterns.
func FindFiles(globs ...string) ([]string, error) {
	var configFiles []string
	for _, glob := range globs {
		files, err := filepath.Glob(glob)
		if err != nil {
			return nil, errors.Wrapf(err, "failed on glob %v", glob)
		}
		configFiles = append(configFiles, files...)
	}
	return configFiles, nil
}
