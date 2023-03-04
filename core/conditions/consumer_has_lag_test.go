/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConsumerHasLagCondition(t *testing.T) {
	config := Config{
		ConsumerHasLag: &Fields{fields: map[string]interface{}{
			"queue":    "myqueue",
			"group":    "group",
			"consumer": "myconsumer1",
		}},
	}
	_, err := NewCondition(&config)
	assert.NoError(t, err)
}
