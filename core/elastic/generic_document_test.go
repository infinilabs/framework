package elastic

import "testing"

func TestSearchResponseWithMeta_GetTotal(t *testing.T) {
	type dummyDoc struct{}

	tests := []struct {
		name     string
		response *SearchResponseWithMeta[dummyDoc]
		want     int64
	}{
		{
			name:     "nil response",
			response: nil,
			want:     -1,
		},
		{
			name:     "nil total",
			response: &SearchResponseWithMeta[dummyDoc]{},
			want:     -1,
		},
		{
			name: "ES 7+ total as map with value",
			response: &SearchResponseWithMeta[dummyDoc]{
				Hits: SearchHits[dummyDoc]{
					Total: map[string]interface{}{
						"value":    float64(123),
						"relation": "eq",
					},
				},
			},
			want: 123,
		},
		{
			name: "total as int64",
			response: &SearchResponseWithMeta[dummyDoc]{
				Hits: SearchHits[dummyDoc]{
					Total: int64(456),
				},
			},
			want: 456,
		},
		{
			name: "total as int",
			response: &SearchResponseWithMeta[dummyDoc]{
				Hits: SearchHits[dummyDoc]{
					Total: int(789),
				},
			},
			want: 789,
		},
		{
			name: "total as float64",
			response: &SearchResponseWithMeta[dummyDoc]{
				Hits: SearchHits[dummyDoc]{
					Total: float64(101),
				},
			},
			want: 101,
		},
		{
			name: "total as string fallback",
			response: &SearchResponseWithMeta[dummyDoc]{
				Hits: SearchHits[dummyDoc]{
					Total: "202",
				},
			},
			want: 202,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.GetTotal()
			if got != tt.want {
				t.Fatalf("GetTotal() = %d, want %d", got, tt.want)
			}
		})
	}
}
