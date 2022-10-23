/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package env

import "sync"

type HealthType int
const  HEALTH_UNKNOWN HealthType =0
const  HEALTH_GREEN HealthType =1
const  HEALTH_YELLOW HealthType =2
const  HEALTH_RED HealthType=3
const  HEALTH_UNAVAILABLE HealthType=4

func GetHealthType(health string)HealthType  {
	switch health {
	case "green":
		return HEALTH_GREEN
	case "yellow":
		return HEALTH_YELLOW
	case "red":
		return HEALTH_RED
	case "unavailable":
		return HEALTH_UNAVAILABLE
	}
	return HEALTH_UNKNOWN
}

func (h HealthType)ToString()string  {
	switch h {
	case HEALTH_YELLOW:
		return "yellow"
	case HEALTH_RED:
		return "red"
	case HEALTH_GREEN:
		return "green"
	case HEALTH_UNAVAILABLE:
		return "unavailable"
	}
	return "unknown"
}
var h =sync.Map{}

func (env *Env) ReportHealth(service string,health HealthType)  {
	h.Store(service,health)
}

func (env *Env) GetOverallHealth() HealthType {
	t:=HEALTH_GREEN
	h.Range(func(key, value any) bool {
		x:=value.(HealthType)
		if x>t{
			t=x
		}
		return true
	})
	return t
}

func (env *Env) GetServicesHealth() map[string]string {
	o:=map[string]string{}
	h.Range(func(key, value any) bool {
		o[key.(string)]=value.(HealthType).ToString()
		return true
	})
	return o
}