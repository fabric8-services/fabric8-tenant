package utils

import (
	"bytes"
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

	return ioutil.ReadAll(resp.Body)
}

func ReadBody(response *http.Response) (func() []byte, func()) {
	if response == nil {
		return func() []byte {
			return []byte{}
		}, func() {}
	}
	toDefer := func() {
		ioutil.ReadAll(response.Body)
		response.Body.Close()
	}

	readBody := func() []byte {
		buf := new(bytes.Buffer)
		buf.ReadFrom(response.Body)
		return buf.Bytes()
	}
	return readBody, toDefer
}
