/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package entity_card

type EntityInfo struct {
	Type       string       `json:"type,omitempty"`
	ID         string       `json:"id,omitempty"`
	Style      *CardStyle   `json:"style,omitempty"`
	Color      string       `json:"color,omitempty"`
	Icon       string       `json:"icon,omitempty"` //https://lucide.dev/icons/
	Title      string       `json:"title,omitempty"`
	Subtitle   string       `json:"subtitle,omitempty"`
	URL        string       `json:"url,omitempty"`
	Cover      string       `json:"cover,omitempty"`
	Categories []string     `json:"categories,omitempty"`
	Tags       []string     `json:"tags,omitempty"`
	Properties []Property   `json:"properties,omitempty"`
	Details    *CardDetails `json:"details,omitempty"`
}

type EntityLabel struct {
	Type     string `json:"type,omitempty"`
	ID       string `json:"id,omitempty"`
	Color    string `json:"color,omitempty"`
	Icon     string `json:"icon,omitempty"` //https://lucide.dev/icons/
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	URL      string `json:"url,omitempty"`
}

type CardStyle struct {
	Width          string `json:"width,omitempty"`
	Height         string `json:"height,omitempty"`
	MaxWidth       string `json:"max_width,omitempty"`
	MaxHeight      string `json:"max_height,omitempty"`
	CoverMaxHeight string `json:"cover_max_height,omitempty"`
}

type Property struct {
	Icon    string                 `json:"icon,omitempty"`
	Value   interface{}            `json:"value,omitempty"`
	View    string                 `json:"view,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type CardDetails struct {
	Table *CardTable `json:"table,omitempty"`
}

type CardTable struct {
	Rows []CardRow `json:"rows,omitempty"`
}

type CardRow struct {
	Columns []CardColumn `json:"columns,omitempty"`
}

type CardColumn struct {
	Label   string                 `json:"label,omitempty"`
	Value   interface{}            `json:"value,omitempty"`
	View    string                 `json:"view,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}
