package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// DownloadFile retrieves the content of the file on the given url
func DownloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return make([]byte, 0), fmt.Errorf("server responded with error %d", resp.StatusCode)
	}

	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
