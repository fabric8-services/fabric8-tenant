package token

import (
	"net/http"
	"testing"
)

func Test_validateError(t *testing.T) {
	type args struct {
		status int
		body   []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "status ok",
			args:    args{status: http.StatusOK},
			wantErr: false,
		},
		{
			name:    "unmarshalling should fail",
			args:    args{body: []byte("foobar")},
			wantErr: true,
		},
		{
			name: "return proper error",
			args: args{
				status: http.StatusNotFound,
				body: []byte(`
					{
						"errors": [
							{
								"code": "jwt_security_error",
								"detail": "JWT validation failed: token contains an invalid number of segments",
								"id": "BEO45Wxi",
								"status": "401",
								"title": "Unauthorized"
							}
						]
					}`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateError(tt.args.status, tt.args.body); (err != nil) != tt.wantErr {
				t.Errorf("validateError() error = %v, wantErr %v", err, tt.wantErr)
			} else if tt.wantErr && err != nil {
				t.Logf("validateError() failed with error = %v", err)
			}
		})
	}
}

func Test_parseToken(t *testing.T) {
	want := "fake_token"
	output := `
		{
			"access_token": "` + want + `",
			"token_type": "bearer"
		}`

	tests := []struct {
		name    string
		data    []byte
		want    string
		wantErr bool
	}{
		{
			name:    "bad respose so should not parse the output",
			wantErr: true,
		},
		{
			name:    "bad respose so should not parse the output",
			wantErr: true,
			data:    []byte("foobar"),
		},
		{
			name:    "should parse the output to extract token",
			wantErr: false,
			data:    []byte(output),
			want:    want,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseToken(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr && err != nil {
				t.Logf("parseToken() failed with error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
