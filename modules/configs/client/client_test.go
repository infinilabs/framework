package client

import (
	"testing"

	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/configs/common"
)

func TestApplyManagerRequestAuthAddsRegisterTokenHeader(t *testing.T) {
	req := util.Request{Path: common.REGISTER_API}

	applyManagerRequestAuth(&req, "", "", "token-1")

	if got := req.AllHeaders()[common.API_TOKEN]; got != "token-1" {
		t.Fatalf("expected register token header %q, got %q", "token-1", got)
	}
}

func TestApplyManagerRequestAuthAddsTokenHeaderForSync(t *testing.T) {
	req := util.Request{Path: common.SYNC_API}

	applyManagerRequestAuth(&req, "", "", "token-1")

	if got := req.AllHeaders()[common.API_TOKEN]; got != "token-1" {
		t.Fatalf("expected sync token header %q, got %q", "token-1", got)
	}
}
