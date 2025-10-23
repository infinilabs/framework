/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import "testing"

func TestGetPermissionItemParsing(t *testing.T) {
	tests := []struct {
		input    string
		category string
		resource string
		action   string
	}{
		// --- basic patterns ---
		{"coco#home/view", "coco", "home", "view"},
		{"coco#home/", "coco", "home", ""},
		{"coco#home", "coco", "home", ""},
		{"coco#/view", "coco", "", "view"},

		// --- with colons (:) in different parts ---
		{"ai:team#model:gpt4/run", "ai:team", "model:gpt4", "run"},
		{"ai:team#model:gpt4/run:v2", "ai:team", "model:gpt4", "run:v2"},
		{"ai#service:v1/test", "ai", "service:v1", "test"},
		{"search:prod#index:v1/query", "search:prod", "index:v1", "query"},

		// --- no category or no action ---
		{"#config/update", "", "config", "update"},
		{"coco#", "coco", "", ""},
		{"#/view", "", "", "view"},
		{"system#config", "system", "config", ""},

		// --- multiple delimiters, ensure only first # and / are used ---
		{"app#res:part/sub:section/action:deep", "app", "res:part", "sub:section/action:deep"},
		{"x:y:z#a:b:c/d:e:f", "x:y:z", "a:b:c", "d:e:f"},

		// --- no delimiters ---
		{"plain", "", "plain", ""},
		{"no/slash", "", "no", "slash"},
		{"no#hash", "no", "hash", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			category, resource, action := parsePermissionKey(tt.input)
			if category != tt.category || resource != tt.resource || action != tt.action {
				t.Errorf("parse(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.input, category, resource, action, tt.category, tt.resource, tt.action)
			}
		})
	}
}
