package token

import (
	"os"
	"testing"
)

func TestGetUserCluster(t *testing.T) {

	tests := []struct {
		name    string
		userID  string
		want    string
		wantErr bool
		authURL string
	}{
		{
			name:    "see if we can retrieve cluster info",
			userID:  "3383826c-51e4-401b-9ccd-b898f7e2397d",
			want:    "https://api.starter-us-east-2.openshift.com",
			wantErr: false,
			authURL: "https://auth.openshift.io",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("F8_AUTH_URL", tt.authURL)
			got, err := GetUserCluster(tt.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserCluster() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetUserCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
