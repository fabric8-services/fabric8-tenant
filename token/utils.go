package token

import (
	"fmt"
	"net/http"

	"github.com/fabric8-services/fabric8-tenant/auth"
)

func validateError(c *auth.Client, res *http.Response) error {

	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("404 Not found")
	} else if res.StatusCode != http.StatusOK {

		goaErr, err := c.DecodeJSONAPIErrors(res)
		if err != nil {
			return err
		}
		if len(goaErr.Errors) != 0 {
			var output string
			for _, error := range goaErr.Errors {
				output += fmt.Sprintf("%s: %s %s, %s\n", *error.Title, *error.Status, *error.Code, error.Detail)
			}
			return fmt.Errorf("%s", output)
		}
	}
	return nil
}
