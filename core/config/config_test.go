package config

import (
	"fmt"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/magiconair/properties/assert"
	"github.com/spf13/viper"
	"infini.sh/framework/core/util"
	"os"
	"testing"
)

// Defines struct to read config from
type ExampleConfig struct {
	Counter int32 `config:"counter" validate:"min=0, max=9"`
}

// Defines default config option
var (
	defaultConfig = ExampleConfig{
		Counter: 4,
	}
)

func TestLoadDefaultCfg(t *testing.T) {

	path := "config_test.yml"
	appConfig := defaultConfig // copy default config so it's not overwritten
	config, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	assert.Equal(t, appConfig.Counter, int32(4))
	fmt.Println(appConfig.Counter)

	err = config.Unpack(&appConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	assert.Equal(t, appConfig.Counter, int32(5))

	fmt.Println(appConfig.Counter)

}

type globalConfig struct {
	Modules []*Config `config:"modules"`
	MapConfig []*Config `config:"config_map_array"`
}

type crawlerConfig struct {
	Namespace  string `config:"namespace" validate:"required"`
	LikedCount int    `config:"liked"`
}

var (
	defaultCrawlerConfig = crawlerConfig{
		Namespace:  "default",
		LikedCount: 512,
	}
)

func TestLoadModules(t *testing.T) {
	cfg, _ := LoadFile("config_test.yml")

	config := globalConfig{}

	if err := cfg.Unpack(&config); err != nil {
		fmt.Println(err)
	}

	fmt.Println("map_config:",config)
	for k,v:=range config.MapConfig{
		fmt.Println("key:",k)
		fmt.Println("value:",v)
		if v.HasField("if"){
			fmt.Println("is if filter")
		}
	}


	crawlerCfg := defaultCrawlerConfig

	cf1 := newConfig(t, config.Modules)
	cf1[0].Unpack(&crawlerCfg)

	assert.Equal(t, crawlerCfg.Namespace, "hello world")
	assert.Equal(t, crawlerCfg.LikedCount, 1235)



	parserConfig := struct {
		ID string `config:"parser_id" validate:"required"`
	}{}
	cf1[1].Unpack(&parserConfig)
	fmt.Println(parserConfig.ID)




}

func getModuleName(c *Config) string {
	cfgObj := struct {
		Module string `config:"module"`
	}{}

	if c == nil {
		return ""
	}
	if err := c.Unpack(&cfgObj); err != nil {
		return ""
	}

	return cfgObj.Module
}

func newConfig(t testing.TB, cfgs []*Config) []*Config {
	results := []*Config{}
	for _, cfg := range cfgs {
		//set map for modules and module config
		fmt.Println(getModuleName(cfg))
		fmt.Println(cfg.Enabled(true))
		config, err := NewConfigFrom(cfg)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, config)
	}

	return results
}

func TestMergeCfg(t *testing.T) {
	a1 := map[string]interface{}{
		"arr": []interface{}{1, 2},
	}
	a2 := map[string]interface{}{
		"arr": []interface{}{5, 6},
	}
	t.Logf("a1 %v", a1)
	t.Logf("a2 %v", a2)

	c := ucfg.New()
	opts := []ucfg.Option{
		//ucfg.PathSep("."),
		ucfg.AppendValues,
		//ucfg.FieldAppendValues("arr"),
	}
	err := c.Merge(a1, opts...)
	t.Logf("err %v", err)

	err = c.Merge(a2, opts...)
	t.Logf("err %v", err)

	result := map[string]interface{}{}
	err = c.Unpack(&result)
	t.Logf("merge %v %v", result, err)
}

func TestMergeCfg1(t *testing.T) {
	a1 := map[string]interface{}{
		"arr": []map[string]interface{}{
			map[string]interface{}{
				"abc": "efg",
			},
		},
	}
	a2 := map[string]interface{}{
		"arr": []interface{}{map[string]interface{}{
			"efg": "hij",
		}},
	}
	t.Logf("a1 %v", util.ToJson(a1, true))
	t.Logf("a2 %v", util.ToJson(a2, true))

	c := ucfg.New()
	opts := []ucfg.Option{
		//ucfg.PathSep("."),
		ucfg.AppendValues,
		//ucfg.FieldAppendValues("arr"),
	}
	err := c.Merge(a1, opts...)
	t.Logf("err %v", err)

	err = c.Merge(a2, opts...)
	t.Logf("err %v", err)

	result := map[string]interface{}{}
	err = c.Unpack(&result)
	t.Logf("merge %v %v", util.ToJson(result, true), err)
}

type node map[string]interface{}

type config map[string]interface{}

func defaultViperConfig1() config {
	return config{
		"a1": false,
		"a2": config{
			"url":     "url",
			"workers": 2,
		},
	}
}
func defaultViperConfig2() config {
	return config{
		"a2": config{
			"filename": "viperConfig",
		},
	}
}
func TestMergeViperCfg(t *testing.T) {
	conf1 := defaultViperConfig1()
	if err := viper.MergeConfigMap(conf1); err != nil {
		panic(err)
	}
	conf2 := defaultViperConfig2()
	if err := viper.MergeConfigMap(conf2); err != nil {
		panic(err)
	}
	fmt.Println(viper.AllSettings())
	//fmt.Println(viper.GetStringMapString("a1")["url"])
	//fmt.Println(viper.GetStringMapString("b1")["filename"])
}

func TestMergeFieldHandling(t *testing.T) {

	tests := []struct {
		Name    string
		Configs []interface{}
		Options []ucfg.Option
		Assert  func(t *testing.T, c *Config)
	}{
		{
			"default append w/ replace paths",
			[]interface{}{
				map[string]interface{}{
					"paths": []interface{}{
						"removed_1.log",
						"removed_2.log",
						"removed_2.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_locale": map[string]interface{}{},
						},
					},
				},
				map[string]interface{}{
					"paths": []interface{}{
						"container.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_fields": map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.AppendValues,
				ucfg.FieldReplaceValues("paths"),
			},
			func(t *testing.T, c *Config) {
				unpacked := make(map[string]interface{})
				assert.Equal(t, c.Unpack(unpacked), nil)
				fmt.Println("new:", unpacked)
				paths, _ := unpacked["paths"]
				assert.Equal(t, paths, 1)
				assert.Equal(t, []interface{}{"container.log"}, paths)

				processors, _ := unpacked["processors"]
				assert.Equal(t, processors, 2)

				processorNames := make([]string, 2)
				procs := processors.([]interface{})
				for i, proc := range procs {
					for name := range proc.(map[string]interface{}) {
						processorNames[i] = name
					}
				}
				assert.Equal(t, []string{"add_locale", "add_fields"}, processorNames)
			},
		},
		{
			"default prepend w/ replace paths",
			[]interface{}{
				map[string]interface{}{
					"paths": []interface{}{
						"removed_1.log",
						"removed_2.log",
						"removed_2.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_locale": map[string]interface{}{},
						},
					},
				},
				map[string]interface{}{
					"paths": []interface{}{
						"container.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_fields": map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.PrependValues,
				ucfg.FieldReplaceValues("paths"),
			},
			func(t *testing.T, c *Config) {
				unpacked := make(map[string]interface{})
				assert.Equal(t, c.Unpack(unpacked), nil)

				paths, _ := unpacked["paths"]
				assert.Equal(t, paths, 1)
				assert.Equal(t, []interface{}{"container.log"}, paths)

				processors, _ := unpacked["processors"]
				assert.Equal(t, processors, 2)

				processorNames := make([]string, 2)
				procs := processors.([]interface{})
				for i, proc := range procs {
					for name := range proc.(map[string]interface{}) {
						processorNames[i] = name
					}
				}
				assert.Equal(t, []string{"add_fields", "add_locale"}, processorNames)
			},
		},
		{
			"replace paths and append processors",
			[]interface{}{
				map[string]interface{}{
					"paths": []interface{}{
						"removed_1.log",
						"removed_2.log",
						"removed_2.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_locale": map[string]interface{}{},
						},
					},
				},
				map[string]interface{}{
					"paths": []interface{}{
						"container.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_fields": map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.FieldReplaceValues("paths"),
				ucfg.FieldAppendValues("processors"),
			},
			func(t *testing.T, c *Config) {
				unpacked := make(map[string]interface{})
				assert.Equal(t, c.Unpack(unpacked), nil)

				paths, _ := unpacked["paths"]
				assert.Equal(t, paths, 1)
				assert.Equal(t, []interface{}{"container.log"}, paths)

				processors, _ := unpacked["processors"]
				assert.Equal(t, processors, 2)

				processorNames := make([]string, 2)
				procs := processors.([]interface{})
				for i, proc := range procs {
					for name := range proc.(map[string]interface{}) {
						processorNames[i] = name
					}
				}
				assert.Equal(t, []string{"add_locale", "add_fields"}, processorNames)
			},
		},
		{
			"default append w/ replace paths and prepend processors",
			[]interface{}{
				map[string]interface{}{
					"paths": []interface{}{
						"removed_1.log",
						"removed_2.log",
						"removed_2.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_locale": map[string]interface{}{},
						},
					},
				},
				map[string]interface{}{
					"paths": []interface{}{
						"container.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_fields": map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.AppendValues,
				ucfg.FieldReplaceValues("paths"),
				ucfg.FieldPrependValues("processors"),
			},
			func(t *testing.T, c *Config) {
				unpacked := make(map[string]interface{})
				assert.Equal(t, c.Unpack(unpacked), nil)

				paths, _ := unpacked["paths"]
				assert.Equal(t, paths, 1)
				assert.Equal(t, []interface{}{"container.log"}, paths)

				processors, _ := unpacked["processors"]
				assert.Equal(t, processors, 2)

				processorNames := make([]string, 2)
				procs := processors.([]interface{})
				for i, proc := range procs {
					for name := range proc.(map[string]interface{}) {
						processorNames[i] = name
					}
				}
				assert.Equal(t, []string{"add_fields", "add_locale"}, processorNames)
			},
		},
		{
			"nested replace paths and append processors",
			[]interface{}{
				[]interface{}{
					map[string]interface{}{
						"paths": []interface{}{
							"removed_1.log",
							"removed_2.log",
							"removed_2.log",
						},
						"processors": []interface{}{
							map[string]interface{}{
								"add_locale": map[string]interface{}{},
							},
						},
					},
				},
				[]interface{}{
					map[string]interface{}{
						"paths": []interface{}{
							"container.log",
						},
						"processors": []interface{}{
							map[string]interface{}{
								"add_fields": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.FieldReplaceValues("*.paths"),
				ucfg.FieldAppendValues("*.processors"),
			},
			func(t *testing.T, c *Config) {
				var unpacked []interface{}
				assert.Equal(t, c.Unpack(&unpacked), nil)

				nested := unpacked[0].(map[string]interface{})
				paths, _ := nested["paths"]
				assert.Equal(t, paths, 1)
				assert.Equal(t, []interface{}{"container.log"}, paths)

				processors, _ := nested["processors"]
				assert.Equal(t, processors, 2)

				processorNames := make([]string, 2)
				procs := processors.([]interface{})
				for i, proc := range procs {
					for name := range proc.(map[string]interface{}) {
						processorNames[i] = name
					}
				}
				assert.Equal(t, []string{"add_locale", "add_fields"}, processorNames)
			},
		},
		{
			"deep unknown nested replace paths and append processors",
			[]interface{}{
				[]interface{}{
					map[string]interface{}{
						"deep": []interface{}{
							map[string]interface{}{
								"paths": []interface{}{
									"removed_1.log",
									"removed_2.log",
									"removed_2.log",
								},
								"processors": []interface{}{
									map[string]interface{}{
										"add_locale": map[string]interface{}{},
									},
								},
							},
						},
					},
				},
				[]interface{}{
					map[string]interface{}{
						"deep": []interface{}{
							map[string]interface{}{
								"paths": []interface{}{
									"container.log",
								},
								"processors": []interface{}{
									map[string]interface{}{
										"add_fields": map[string]interface{}{
											"foo": "bar",
										},
									},
								},
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.FieldReplaceValues("**.paths"),
				ucfg.FieldAppendValues("**.processors"),
			},
			func(t *testing.T, c *Config) {
				var unpacked []interface{}
				assert.Equal(t, c.Unpack(&unpacked), nil)

				level0 := unpacked[0].(map[string]interface{})
				deep, _ := level0["deep"].([]interface{})
				nested := deep[0].(map[string]interface{})
				paths, _ := nested["paths"]
				assert.Equal(t, paths, 1)
				assert.Equal(t, []interface{}{"container.log"}, paths)

				processors, _ := nested["processors"]
				assert.Equal(t, processors, 2)

				processorNames := make([]string, 2)
				procs := processors.([]interface{})
				for i, proc := range procs {
					for name := range proc.(map[string]interface{}) {
						processorNames[i] = name
					}
				}
				assert.Equal(t, []string{"add_locale", "add_fields"}, processorNames)
			},
		},
		{
			"replace paths and append processors using depth selector (but fields are at level0)",
			[]interface{}{
				map[string]interface{}{
					"paths": []interface{}{
						"removed_1.log",
						"removed_2.log",
						"removed_2.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_locale": map[string]interface{}{},
						},
					},
				},
				map[string]interface{}{
					"paths": []interface{}{
						"container.log",
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_fields": map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.FieldReplaceValues("**.paths"),
				ucfg.FieldAppendValues("**.processors"),
			},
			func(t *testing.T, c *Config) {
				unpacked := make(map[string]interface{})
				assert.Equal(t, c.Unpack(unpacked), nil)

				paths, _ := unpacked["paths"]
				assert.Equal(t, paths, 1)
				assert.Equal(t, []interface{}{"container.log"}, paths)

				processors, _ := unpacked["processors"]
				assert.Equal(t, processors, 2)

				processorNames := make([]string, 2)
				procs := processors.([]interface{})
				for i, proc := range procs {
					for name := range proc.(map[string]interface{}) {
						processorNames[i] = name
					}
				}
				assert.Equal(t, []string{"add_locale", "add_fields"}, processorNames)
			},
		},
		{
			"adjust merging based on indexes",
			[]interface{}{
				map[string]interface{}{
					"processors": []interface{}{
						map[string]interface{}{
							"add_locale": map[string]interface{}{},
						},
						map[string]interface{}{
							"add_fields": map[string]interface{}{
								"foo": "bar",
							},
						},
						map[string]interface{}{
							"add_tags": map[string]interface{}{
								"tags": []string{"merged"},
							},
						},
					},
				},
				map[string]interface{}{
					"processors": []interface{}{
						map[string]interface{}{
							"add_locale": map[string]interface{}{},
						},
						map[string]interface{}{
							"add_fields": map[string]interface{}{
								"replace": "no-bar",
							},
						},
						map[string]interface{}{
							"add_tags": map[string]interface{}{
								"tags": []string{"together"},
							},
						},
					},
				},
			},
			[]ucfg.Option{
				ucfg.PathSep("."),
				ucfg.FieldReplaceValues("processors.1"),
				ucfg.FieldAppendValues("processors.2.add_tags.tags"),
			},
			func(t *testing.T, c *Config) {
				unpacked := make(map[string]interface{})
				assert.Equal(t, c.Unpack(unpacked), nil)

				processors, _ := unpacked["processors"]
				assert.Equal(t, processors, 3)

				processorsByAction := make(map[string]interface{})
				procs := processors.([]interface{})
				for _, proc := range procs {
					for name, val := range proc.(map[string]interface{}) {
						processorsByAction[name] = val
					}
				}

				addFieldsAction, ok := processorsByAction["add_fields"]
				assert.Equal(t, ok, true)
				assert.Equal(t, map[string]interface{}{"replace": "no-bar"}, addFieldsAction)

				addTagsAction, ok := processorsByAction["add_tags"]
				assert.Equal(t, ok, true)
				tags, ok := (addTagsAction.(map[string]interface{}))["tags"]
				assert.Equal(t, ok, true)
				assert.Equal(t, []interface{}{"merged", "together"}, tags)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			c := ucfg.New()
			for _, config := range test.Configs {
				assert.Equal(t, c.Merge(config, test.Options...), nil)
			}
			//test.Assert(t, c)
		})
	}
}
