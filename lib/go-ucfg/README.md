[![Build Status](https://beats-ci.elastic.co/job/Library/job/go-ucfg-mbp/job/main/badge/icon)](https://beats-ci.elastic.co/job/Library/job/go-ucfg-mbp/job/main/)
[![Go Report
Card](https://goreportcard.com/badge/infini.sh/framework/lib/go-ucfg)](https://goreportcard.com/report/infini.sh/framework/lib/go-ucfg)
[![codecov](https://codecov.io/gh/elastic/go-ucfg/branch/main/graph/badge.svg)](https://codecov.io/gh/elastic/go-ucfg)


# ucfg - Universal Configuration

`ucfg` is a Golang library to handle hjson, json, and yaml configuration files in your Golang project. It was developed for the [libbeat framework](https://github.com/elastic/beats/tree/master/libbeat) and used by all [beats](https://github.com/elastic/beats).


## API Documentation

The full API Documentation can be found [here](https://godoc.org/infini.sh/framework/lib/go-ucfg).

## Examples

A few examples on how ucfg can be used. All examples below assume, that the following packages are imported:

```golang
import (
	"infini.sh/framework/lib/go-ucfg"
	"infini.sh/framework/lib/go-ucfg/yaml"
)
```

### Dot notations

ufcg allows you to load yaml configuration files using dots instead of indentation. For example instead of having:

```yaml
config:
  user: name
```

with ucfg you can write:

```yaml
config.user: name
```

This makes configurations easier and simpler.

To load such a config file in Golang, use the following command:

```golang
config, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
```



### Validation and Defaults

ucfg allows to automatically validate fields and set defaults for fields in case they are not defined.


```golang
// Defines struct to read config from
type ExampleConfig struct {
    Counter  int 	`config:"counter" validate:"min=0, max=9"`
}

// Defines default config option
var (
    defaultConfig = ExampleConfig{
		    Counter: 4,
    }
)

func main() {
    appConfig := defaultConfig // copy default config so it's not overwritten
    config, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    err = config.Unpack(&appConfig)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
```

The above uses `Counter` as the config variable. ucfg assures that the value is between 0 and 9 and will return an error if this is not the case. In addition, if the value is not set, it will default to 4.


## Requirements

ucfg has the following requirements:

* Golang 1.10+
