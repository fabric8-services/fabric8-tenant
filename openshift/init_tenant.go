package openshift

import (
	"fmt"
	"strings"

	"github.com/fabric8io/fabric8-init-tenant/template"
)

// InitTenant initializes a new tenant in openshift
// Creates the new n-tuneim|develop,ment|staging and x-dsaas-* namespaces
// and install the required services/routes/deployment configurations to run
// e.g. Jenkins and Che
func InitTenant(config Config, username string) error {
	err := do(config, username)
	if err != nil {
		return err
	}
	return nil
}

func do(config Config, username string) error {
	namespaces := []string{
		"%s-development",
		"%s-testing",
		"%s-staging",
		"%s-runtime",
	}

	name := createName(username)

	vars := map[string]string{
		"NAME":                    name,
		"PROJECT_DISPLAYNAME":     name + " Test Project",
		"PROJECT_DESCRIPTION":     name + " Test Project",
		"PROJECT_REQUESTING_USER": username,
		"PROJECT_ADMIN_USER":      username,
	}

	opts := ApplyOptions{Config: config}

	projectT, err := template.Asset("fabric8-online.yaml")
	if err != nil {
		return err
	}

	contentRepoT, err := template.Asset("content-repository-2.2.330-openshift.yaml")
	if err != nil {
		return err
	}

	jenkinsT, err := template.Asset("jenkins-openshift-2.2.330-openshift.yaml")
	if err != nil {
		return err
	}

	cheT, err := template.Asset("che-1.0.58-openshift.yaml")
	if err != nil {
		return err
	}

	var channels []chan error

	for _, pattern := range namespaces {
		lvars := clone(vars)
		lvars["PROJECT_NAME"] = fmt.Sprintf(pattern, vars["NAME"])
		lvars["PROJECT_DISPLAYNAME"] = lvars["PROJECT_NAME"]

		ns := createNamespace(string(projectT), lvars, opts)
		channels = append(channels, ns)
	}

	{
		lvars := clone(vars)
		lvars["PROJECT_NAME"] = vars["NAME"] + "-dsaas-jenkins"
		lvars["PROJECT_DISPLAYNAME"] = lvars["PROJECT_NAME"]

		ns := createJenkinsNamespace(string(projectT), string(jenkinsT), string(contentRepoT), lvars, opts)
		channels = append(channels, ns)
	}
	{
		lvars := clone(vars)
		lvars["PROJECT_NAME"] = vars["NAME"] + "-dsaas-che"
		lvars["PROJECT_DISPLAYNAME"] = lvars["PROJECT_NAME"]

		ns := createCheNamespace(string(projectT), string(cheT), lvars, opts)
		channels = append(channels, ns)
	}

	var errors []error
	for _, channel := range channels {
		err := <-channel
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return multiError{Errors: errors}
	}
	return nil
}

func createName(username string) string {
	return strings.Split(username, "@")[0]
}

func createNamespace(template string, vars map[string]string, opts ApplyOptions) chan error {
	ch := make(chan error)
	go func() {
		t, err := Process(template, vars)
		if err != nil {
			ch <- err
		}

		err = Apply(t, opts)
		if err != nil {
			ch <- err
		}

		ch <- nil
		close(ch)
	}()
	return ch
}

func createJenkinsNamespace(template, jenkinsTemplate, contentRepoTemplate string, vars map[string]string, opts ApplyOptions) chan error {
	ch := make(chan error)
	go func() {
		lopts := ApplyOptions{
			Config:    opts.Config,
			Namespace: vars["PROJECT_NAME"],
		}

		t, err := Process(template, vars)
		if err != nil {
			ch <- err
		}

		err = Apply(t, lopts)
		if err != nil {
			ch <- err
		}
		jch := func() chan error {
			ch := make(chan error)
			go func() {
				err = Apply(jenkinsTemplate, lopts)
				if err != nil {
					ch <- err
				}
				close(ch)
			}()
			return ch
		}()
		cch := func() chan error {
			ch := make(chan error)
			go func() {
				err = Apply(contentRepoTemplate, lopts)
				if err != nil {
					ch <- err
				}
				close(ch)
			}()
			return ch
		}()
		err = <-jch
		if err != nil {
			ch <- err
		}
		err = <-cch
		if err != nil {
			ch <- err
		}

		close(ch)
	}()
	return ch
}

func createCheNamespace(template, cheTemplate string, vars map[string]string, opts ApplyOptions) chan error {
	ch := make(chan error)
	go func() {
		lopts := ApplyOptions{
			Config:    opts.Config,
			Namespace: vars["PROJECT_NAME"],
		}

		t, err := Process(template, vars)
		if err != nil {
			ch <- err
		}

		err = Apply(t, lopts)
		if err != nil {
			ch <- err
		}
		jch := func() chan error {
			ch := make(chan error)
			go func() {
				err = Apply(cheTemplate, lopts)
				if err != nil {
					ch <- err
				}
				close(ch)
			}()
			return ch
		}()
		err = <-jch
		if err != nil {
			ch <- err
		}

		close(ch)
	}()
	return ch
}

func clone(maps map[string]string) map[string]string {
	maps2 := make(map[string]string)
	for k2, v2 := range maps {
		maps2[k2] = v2
	}
	return maps2
}
