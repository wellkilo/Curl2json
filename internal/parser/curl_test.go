package parser

import (
	"testing"

	"caseurl2md/internal/config"
)

func TestCurlParser_Parse(t *testing.T) {
	parser := New()

	tests := []struct {
		name    string
		curl    string
		want    *config.RequestInfo
		wantErr bool
	}{
		{
			name: "简单GET请求",
			curl: "curl http://example.com",
			want: &config.RequestInfo{
				Method:  "GET",
				URL:     "http://example.com",
				Headers: make(map[string]string),
				Body:    "",
			},
			wantErr: false,
		},
		{
			name: "带引号的URL",
			curl: `curl "http://example.com/api"`,
			want: &config.RequestInfo{
				Method:  "GET",
				URL:     "http://example.com/api",
				Headers: make(map[string]string),
				Body:    "",
			},
			wantErr: false,
		},
		{
			name: "POST请求",
			curl: `curl -X POST http://example.com/api -H "Content-Type: application/json" --data '{"key": "value"}'`,
			want: &config.RequestInfo{
				Method:  "POST",
				URL:     "http://example.com/api",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"key": "value"}`,
			},
			wantErr: false,
		},
		{
			name:    "空cURL命令",
			curl:    "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.curl)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.Method != tt.want.Method {
				t.Errorf("Parse() Method = %v, want %v", got.Method, tt.want.Method)
			}
			if got.URL != tt.want.URL {
				t.Errorf("Parse() URL = %v, want %v", got.URL, tt.want.URL)
			}
			if got.Body != tt.want.Body {
				t.Errorf("Parse() Body = %v, want %v", got.Body, tt.want.Body)
			}

			// 比较Headers
			if len(got.Headers) != len(tt.want.Headers) {
				t.Errorf("Parse() Headers length = %v, want %v", len(got.Headers), len(tt.want.Headers))
			}
			for k, v := range tt.want.Headers {
				if got.Headers[k] != v {
					t.Errorf("Parse() Headers[%s] = %v, want %v", k, got.Headers[k], v)
				}
			}
		})
	}
}