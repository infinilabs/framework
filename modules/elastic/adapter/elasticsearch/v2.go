package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"infini.sh/framework/core/util"
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