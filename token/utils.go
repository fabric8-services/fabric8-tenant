package token

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

func validateError(status int, body []byte) error {
	type authEerror struct {
		Code   string `json:"code,omitempty"`
		Detail string `json:"detail,omitempty"`
		Status string `json:"status,omitempty"`
		Title  string `json:"title,omitempty"`
	}

	type errorResponse struct {
		Errors []authEerror `json:"errors,omitempty"`
	}

	if status != http.StatusOK {
		var e errorResponse
		err := json.Unmarshal(body, &e)
		if err != nil {
			return errors.Wrapf(err, "could not unmarshal the response")
		}

		var output string
		for _, error := range e.Errors {
			output += fmt.Sprintf("%s: %s %s, %s\n", error.Title, error.Status, error.Code, error.Detail)
		}
		return fmt.Errorf("%s", output)
	}
	return nil
}

func parseToken(data []byte) (string, error) {
	// this struct is defined to obtain the accesstoken from the output
	type authAccessToken struct {
		AccessToken string `json:"access_token,omitempty"`
	}

	var r authAccessToken
	err := json.Unmarshal(data, &r)
	if err != nil {
		return "", errors.Wrapf(err, "error unmarshalling the response")
	}
	return strings.TrimSpace(r.AccessToken), nil
}
