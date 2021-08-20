package elastic

import "time"

type CommandRequest struct {
	Path string `json:"path"`
	Method string `json:"method"`
	Body string `json:"body"`
}

type CommonCommand struct {
	ID string `json:"-" index:"id"`
	Title string `json:"title" elastic_mapping:"title:{type:text}"`
	Tag []string `json:"tag" elastic_mapping:"tag:{type:object}`
	Requests []CommandRequest `json:"requests" elastic_mapping:"tag:{type:object}`
	Created     time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
}