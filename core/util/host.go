package util

import "os"

func GetHostName()string  {
	v,_:=os.Hostname()
	return v
}