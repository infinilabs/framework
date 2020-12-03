/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipeline

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
	"testing"
	"time"
)

type crawlerJoint struct {
	param.Parameters
}

func (joint crawlerJoint) Name() string {
	return "crawler"
}

func (joint crawlerJoint) Process(s *Context) error {
	s.Data[("webpage")] = "hello world gogo "
	s.Data["received_url"] = joint.Data["url"]
	s.Data[("status")] = true
	fmt.Println("start to crawlling url: ", joint.Get("url"))
	return nil
}

type parserJoint struct {
}

func (joint parserJoint) Name() string {
	return "parser"
}

func (joint parserJoint) Process(s *Context) error {
	s.Data[("urls")] = "gogo"
	s.Data[("host")] = "http://gogo.com"
	//pub urls to channel
	fmt.Println("start to parse web content")
	return nil
}

type saveJoint struct {
}

func (joint saveJoint) Name() string {
	return "save"
}

func (joint saveJoint) Process(s *Context) error {
	s.Set("saved", "true")
	//pub urls to channel
	fmt.Println("start to save web content")
	return nil
}

type publishJoint struct {
}

func (joint publishJoint) Name() string {
	return "publish"
}

func (joint publishJoint) Process(s *Context) error {
	fmt.Println("start to end pipeline")
	s.Set("published", "true")
	return nil
}

func TestPipeline(t *testing.T) {

	global.RegisterEnv(env.EmptyEnv())

	pipeline := NewPipeline("crawler_test")
	context := &Context{}
	context.Set("url", "gogol.com")
	context.Set("webpage", "hello world gogo ")

	crawler := crawlerJoint{}

	pipeline.Context(context).
		Start(crawler).
		Join(parserJoint{}).
		Join(saveJoint{}).
		Join(publishJoint{}).
		Run()

	fmt.Println(context.Data)
	fmt.Println(context.Data)
	assert.Equal(t, "true", context.Data["published"])
	assert.Equal(t, "true", context.Data["saved"])
	assert.Equal(t, true, context.Data["status"])
	assert.Equal(t, "http://gogo.com", context.Data["host"])
}

const key1 param.ParaKey = "DEPTH"
const key2 param.ParaKey = "DEPTH2"

func TestContext(t *testing.T) {
	global.RegisterEnv(env.EmptyEnv())
	context := &Context{}
	context.Set(key1, 23)
	fmt.Println(util.ToJson(context, true))
	v := context.MustGetInt(key1)
	assert.Equal(t, 23, v)
	v, _ = context.GetInt(key2, 0)
	assert.Equal(t, 0, v)
}

func TestContextGetBytes(t *testing.T) {
	global.RegisterEnv(env.EmptyEnv())
	context := &Context{}
	v := []byte("hello")
	context.Set(key1, v)
	v1 := context.MustGetBytes(key1)
	fmt.Println(v1)
	assert.Equal(t, v, v1)
}

func TestContextMarshal(t *testing.T) {
	global.RegisterEnv(env.EmptyEnv())
	url := "http://google.com"
	context := Context{IgnoreBroken: true}
	context.Set("URL", url)
	arr := []byte(url)
	fmt.Println(arr)
	context.Set("B", arr)
	fmt.Println("before:", context)

	c1 := util.ToJSONBytes(context)

	fmt.Println(string(c1))

	c2 := context.Marshall()
	fmt.Println(c2)

	assert.Equal(t, c1, c2)

	c := Context{}
	util.FromJSONBytes(c1, &c)
	fmt.Println("after:", c)
	assert.Equal(t, url, c.Get("URL"))

	b2, _ := c.GetBytes("B")
	fmt.Println("new B:", string(b2))
	assert.Equal(t, []byte(url), b2)

	c = UnMarshall(c1)
	fmt.Println("after:", c)
	assert.Equal(t, url, c.Get("URL"))

	b2, _ = c.GetBytes("B")
	fmt.Println("new B:", string(b2))
	assert.Equal(t, []byte(url), b2)

}

type myinput struct {
}

func (input myinput) Open() error {
	fmt.Println("opening input now")
	return nil
}

func (input myinput) Close() error {
	fmt.Println("closing input now")
	return nil
}

func (input myinput) Read() ([]byte, error) {
	time.Sleep(1 * time.Second)
	fmt.Println("reading now")
	return []byte("a"), nil
}

type myoutput struct {
}

func (output myoutput) Open() error {
	fmt.Println("opening output now")
	return nil
}

func (output myoutput) Close() error {
	fmt.Println("closing output now")
	return nil
}

func (output myoutput) Write(data []byte) error {
	fmt.Println("writing now,", string(data))
	return nil
}

type SimpleFilter struct {
}

func (filter SimpleFilter) Filter(data []byte) error {
	fmt.Println("filter now,", util.ToUppercase(data))
	return nil
}

func TestPipelineReadingDataAndOutput(t *testing.T) {

	global.RegisterEnv(env.EmptyEnv())

	filters := []Filter{SimpleFilter{}}

	context := &Context{}
	pipe := Pipeline{input: myinput{}, output: myoutput{}, context: context, filters: filters}

	pipe.Start1()
	var key param.ParaKey = "KEY"
	i := 0

	context.Set(key, i)

	go func() {
		time.Sleep(10 * time.Second)
		pipe.Pause()
		fmt.Println("paused pipeline")
		time.Sleep(10 * time.Second)
		pipe.Resume()
		fmt.Println("resumed pipeline")
		time.Sleep(10 * time.Second)
		pipe.Stop()
		fmt.Println("stopped pipeline")
	}()

	var err error
	err = pipe.input.Open()
	if err != nil {
		panic(err)
	}
	defer pipe.input.Close()

	pipe.output.Open()
	defer pipe.output.Close()

	var data []byte

	for {
		switch pipe.runningState {
		case STARTED:
			data, err = pipe.input.Read()
			if err != nil {
				fmt.Println(err)

				break
			}
			x := pipe.context.MustGetInt(key)
			fmt.Println("x:", x)
			x++
			pipe.context.Set(key, x)

			for _, f := range pipe.filters {
				err = f.Filter(data)
				if err != nil {
					panic(err)
				}
			}

			err = pipe.output.Write(data)
			if err != nil {
				panic(err)
			}

			if i > 100 {
				break
			}
			break
		case PAUSED:
			break
		case STOPPED:
			return
		}

	}

}
