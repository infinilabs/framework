// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"infini.sh/framework/core/errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

	//// Get the current directories Go import path.
	//repo, err := devtools.GetProjectRepoInfo()
	//if err != nil {
	//	log.Fatalf("Failed to determine import path: %v", err)
	//}

	// Build import paths.
	var imports []string
	for _, dir := range pluginDirs {

		//fmt.Println("handling dir:",dir)
		// Skip packages without an init() function because that cannot register
		// anything as a side-effect of being imported (e.g. filebeat/input/file).

		//var foundInitMethod bool
		//goFiles, err := filepath.Glob(filepath.Join(dir, "*.go"))

		libRegEx, e := regexp.Compile(".*.go$")
		if e != nil {
			log.Fatal(e)
		}

		e = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {

			//fmt.Println(info.Name())
			if err == nil && libRegEx.MatchString(info.Name()) {

				if strings.HasSuffix(info.Name(), "_test.go") {
					return nil
				}
				if hasInitMethod(filepath.Join(path)) {
					//foundInitMethod = true
					fmt.Println(path)
					//fmt.Println(info.Name())

					imports = append(imports, filepath.ToSlash(
						filepath.Join(importPrefix, filepath.Dir(path))))

					return nil
				}
			}
			return nil
		})
		if e != nil {
			log.Fatal(e)
		}


		//
		//goFiles, err := filepath.Glob(filepath.Join(dir, "*.go"))
		//if err != nil {
		//	log.Fatalf("Failed checking for .go files in package dir: %v", err)
		//}
		//for _, f := range goFiles {
		//	fmt.Println("go:",f)
		//	// Skip test files
		//	if strings.HasSuffix(f, "_test.go") {
		//		continue
		//	}
		//	if hasInitMethod(f) {
		//		foundInitMethod = true
		//		break
		//	}
		//}
		//if !foundInitMethod {
		//	continue
		//}

		//importDir := dir
		//if filepath.IsAbs(dir) {
		//	// Make it relative to the current package if it's absolute.
		//	importDir, err = filepath.Rel(devtools.CWD(), dir)
		//	if err != nil {
		//		log.Fatalf("Failure creating import for dir=%v: %v", dir, err)
		//	}
		//}

		//imports = append(imports, filepath.ToSlash(
		//	filepath.Join(repo.ImportPath, importDir)))
		//
		//imports = append(imports, filepath.ToSlash(
		//	filepath.Join("", importDir)))
	}

	sort.Strings(imports)

	fmt.Println("imports:",imports)

	// Populate the template.
	var buf bytes.Buffer
	err := Template.Execute(&buf, Data{
		Package:   pkg,
		Imports:   imports,
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
