package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter"
	log "github.com/cihub/seelog"
	"time"
)

type ESAPIV2 struct {
	ESAPIV0
}

func (s *ESAPIV2) ClearScroll(scrollId string) error {
	url := fmt.Sprintf("%s/_search/scroll", s.GetEndpoint())
	body := util.MustToJSONBytes(util.MapStr{"scroll_id": []string{scrollId}})

	resp, err := s.Request(context.Background(), util.Verb_DELETE, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(string(resp.Body))
	}
	return nil
}

func (s *ESAPIV2) NextScroll(ctx *elastic.APIContext, scrollTime string, scrollId string) ([]byte, error) {

	url := fmt.Sprintf("%s/_search/scroll", s.GetEndpoint())
	body:=util.MapStr{}
	body["scroll_id"]=scrollId
	body["scroll"]=scrollTime

	resp, err := adapter.RequestTimeout(ctx, util.Verb_POST, url, nil, s.metadata, time.Duration(s.metadata.Config.RequestTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	if global.Env().IsDebug {
		log.Trace("next scroll,", url, "m,", string(resp.Body))
	}

	return resp.Body, nil
}