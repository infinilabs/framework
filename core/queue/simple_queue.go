/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/stats"
	"time"
)


func Push(k *QueueConfig, v []byte) error {
	var err error = nil
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getHandler(k)
	if handler != nil {
		err = handler.Push(k.Id, v)
		if err == nil {
			stats.Increment("queue", k.Id, "push")
			return nil
		}
		stats.Increment("queue", k.Id, "push_error")
		return err
	}
	panic(errors.Errorf("handler for [%v] is not registered", k))
}

func Pop(k *QueueConfig) ([]byte, error) {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		//if pausedReadQueue.Contains(k) {
		//	return nil, pauseMsg
		//}

		o, timeout := handler.Pop(k.Id, -1)
		if !timeout {
			stats.Increment("queue", k.Id, "pop")
			return o, nil
		}
		if global.Env().IsDebug {
			stats.Increment("queue", k.Id, "pop_timeout")
		}
		return o, errors.New("timeout")
	}
	panic(errors.New("handler is not registered"))
}

func PopTimeout(k *QueueConfig, timeoutInSeconds time.Duration) (data []byte, timeout bool, err error) {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	if timeoutInSeconds < 1 {
		timeoutInSeconds = 5
	}

	handler := getHandler(k)

	if handler != nil {
		//if pausedReadQueue.Contains(k) {
		//	return nil, false, pauseMsg
		//}

		o, timeout := handler.Pop(k.Id, timeoutInSeconds)
		if !timeout {
			stats.Increment("queue", k.Id, "pop")
		}

		if global.Env().IsDebug {
			stats.Increment("queue", k.Id, "pop_timeout")
		}
		return o, timeout, nil
	}
	panic(errors.New("handler is not registered"))
}


func Depth(k *QueueConfig) int64 {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		o := handler.Depth(k.Id)
		if global.Env().IsDebug {
			stats.Increment("queue", k.Id, "call_depth")
		}
		return o
	}
	panic(errors.New("handler is not registered"))
}
