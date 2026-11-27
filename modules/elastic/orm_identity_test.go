// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package elastic

import (
	"testing"

	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

type identityTestClient struct {
	elastic.API
	response *elastic.GetResponse
}

func (c *identityTestClient) Get(string, string, string) (*elastic.GetResponse, error) {
	return c.response, nil
}

// TestDocumentAbsence distinguishes missing documents from index failures.
func TestDocumentAbsence(t *testing.T) {
	for _, tc := range []struct {
		body    string
		missing bool
	}{
		{`{"_id":"missing","found":false}`, true},
		{`{"error":{"type":"index_not_found_exception"},"status":404}`, false},
		{`{}`, false},
	} {
		response := &elastic.GetResponse{}
		require.NoError(t, util.FromJSONBytes([]byte(tc.body), response))
		response.RawResult = &util.Result{StatusCode: 404, Body: []byte(tc.body)}
		backend := &ElasticORM{Client: &identityTestClient{response: response}}
		account := &security.UserAccount{}
		account.ID = "missing"
		found, err := backend.Get(orm.NewContext(), account)
		require.False(t, found)
		if tc.missing {
			require.ErrorIs(t, err, orm.ErrNotFound)
		} else {
			require.Error(t, err)
			require.NotErrorIs(t, err, orm.ErrNotFound)
		}
	}
}
