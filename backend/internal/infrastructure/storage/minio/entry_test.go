package minio

import (
	"IM_backend/configs"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPresignedURLsUseConfiguredEndpoint(t *testing.T) {
	for _, publicEndpoint := range []string{"", "http://browser.example:9000"} {
		t.Run(publicEndpoint, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			internalHost := strings.TrimPrefix(server.URL, "http://")
			storage, err := NewObjectStorage(context.Background(), configs.MinIOConfig{
				Endpoint: internalHost, PublicEndpoint: publicEndpoint,
				AccessKeyID: "test-access", SecretAccessKey: "test-secret", Bucket: "imchat", Region: "us-east-1",
			})
			if err != nil {
				t.Fatal(err)
			}
			wantHost := internalHost
			if publicEndpoint != "" {
				wantHost = "browser.example:9000"
			}
			ctx := context.Background()
			calls := []struct {
				name string
				run  func() (string, error)
			}{
				{"put", func() (string, error) { return storage.PresignedPutURL(ctx, "uploads/test", time.Minute) }},
				{"get", func() (string, error) {
					return storage.PresignedGetURL(ctx, "uploads/test", time.Minute, "text/plain", "test.txt")
				}},
				{"part", func() (string, error) {
					return storage.PresignMultipartPart(ctx, "uploads/test", "storage-upload-id", 2, time.Minute)
				}},
			}
			for _, call := range calls {
				raw, err := call.run()
				if err != nil {
					t.Fatalf("%s: %v", call.name, err)
				}
				parsed, err := url.Parse(raw)
				if err != nil || parsed.Host != wantHost || parsed.Query().Get("X-Amz-Signature") == "" {
					t.Fatalf("%s 预签名地址无效：%s，错误=%v", call.name, raw, err)
				}
				if call.name == "part" && (parsed.Query().Get("uploadId") != "storage-upload-id" || parsed.Query().Get("partNumber") != "2") {
					t.Fatalf("分片签名参数丢失：%s", raw)
				}
			}
		})
	}
}

func TestParseEndpointUsesExplicitOrDefaultScheme(t *testing.T) {
	for _, test := range []struct {
		raw    string
		secure bool
		want   bool
	}{
		{"minio.example:9000", false, false},
		{"minio.example:9000", true, true},
		{"http://minio.example:9000", true, false},
		{"https://minio.example:9000", false, true},
	} {
		host, secure, err := parseEndpoint(test.raw, test.secure)
		if err != nil || host != "minio.example:9000" || secure != test.want {
			t.Fatalf("解析 %s：host=%s secure=%v err=%v", test.raw, host, secure, err)
		}
	}
}
