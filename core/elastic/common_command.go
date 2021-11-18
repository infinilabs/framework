package elastic

import "time"

type CommandRequest struct {
	Path string `json:"path"`
	Method string `json:"method"`
	Body string `json:"body"`
}

type CommonCommand struct {
	ID string `json:"-" index:"id"`
	Title string `json:"title" elastic_mapping:"title:{type:text,fields:{keyword:{type:keyword}}}"`
	Tag []string `json:"tag" elastic_mapping:"tag:{type:keyword}"`
	Requests []CommandRequest `json:"requests" elastic_mapping:"requests:{type:object}"`
	Created     time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
}