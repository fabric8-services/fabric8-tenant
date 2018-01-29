package token_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/token"
)

func TestClusterCache(t *testing.T) {
	target := "A"

	c := token.NewCachedClusterResolver([]*token.Cluster{
		{APIURL: "X"},
		{APIURL: target},
	})

	found, err := c(context.Background(), target+"/")
	if err != nil {
		t.Error(err)
	}
	if found.APIURL != target {
		t.Errorf("found wrong cluster %v, expected %v", found.APIURL, target)
	}
}
func TestClusterResolver(t *testing.T) {
	tests := []struct {
		name   string
		status int
		count  int
		output string
	}{
		{
			name:   "Fully complete",
			status: 200,
			count:  1,
			output: `{"data": [{"api-url": "http://a.com", "app-dns": "b.com"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if tt.output != "" {
					w.Write([]byte(tt.output))
				}
			}))

			defer ts.Close()

			os.Setenv("F8_AUTH_URL", ts.URL)

			config, err := configuration.GetData()
			if err != nil {
				t.Fatal(err)
			}
			tr := func(ctx context.Context, target, token string, decode token.Decode) (user, accessToken string, err error) {
				return "", "", nil
			}

			cresolver := token.NewAuthClusterClient(config, "aa", tr, token.PlainTextToken)
			clusters, err := cresolver.Get(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(clusters) != tt.count {
				t.Errorf("Wrong number of clusters, got %v but expected %v", len(clusters), tt.count)
			}
		})
	}
}
