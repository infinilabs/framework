/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"infini.sh/framework/core/pipeline"
)

// The codec registry lives in core/pipeline (engine level) so lightweight
// consumers — domain codec modules with a strict dependency surface — can
// register without importing the modules tree. This file re-exports the
// API for existing importers of this package.

type (
	RecordCodec = pipeline.RecordCodec
	CodecInfo   = pipeline.CodecInfo
)

var (
	RegisterRecordCodec          = pipeline.RegisterRecordCodec
	MustRegisterRecordCodec      = pipeline.MustRegisterRecordCodec
	RegisterRecordCodecOrReplace = pipeline.RegisterRecordCodecOrReplace
	GetRecordCodec               = pipeline.GetRecordCodec
	ListRecordCodecs             = pipeline.ListRecordCodecs
)
