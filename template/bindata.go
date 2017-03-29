package template

import (
	"fmt"
	"io/ioutil"
	"strings"
)

// bindata_read reads the given file from disk. It returns an error on failure.
func bindata_read(path, name string) ([]byte, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("Error reading asset %s at %s: %v", name, path, err)
	}
	return buf, err
}

// fabric8_online_che_openshift_yml reads file data from disk. It returns an error on failure.
func fabric8_online_che_openshift_yml() ([]byte, error) {
	return bindata_read(
		"/home/aslak/.gvm/pkgsets/go1.7.3/almighty/src/github.com/fabric8io/fabric8-init-tenant/template/fabric8-online-che-openshift.yml",
		"fabric8-online-che-openshift.yml",
	)
}

// fabric8_online_jenkins_openshift_yml reads file data from disk. It returns an error on failure.
func fabric8_online_jenkins_openshift_yml() ([]byte, error) {
	return bindata_read(
		"/home/aslak/.gvm/pkgsets/go1.7.3/almighty/src/github.com/fabric8io/fabric8-init-tenant/template/fabric8-online-jenkins-openshift.yml",
		"fabric8-online-jenkins-openshift.yml",
	)
}

// fabric8_online_project_openshift_yml reads file data from disk. It returns an error on failure.
func fabric8_online_project_openshift_yml() ([]byte, error) {
	return bindata_read(
		"/home/aslak/.gvm/pkgsets/go1.7.3/almighty/src/github.com/fabric8io/fabric8-init-tenant/template/fabric8-online-project-openshift.yml",
		"fabric8-online-project-openshift.yml",
	)
}

// fabric8_online_team_openshift_yml reads file data from disk. It returns an error on failure.
func fabric8_online_team_openshift_yml() ([]byte, error) {
	return bindata_read(
		"/home/aslak/.gvm/pkgsets/go1.7.3/almighty/src/github.com/fabric8io/fabric8-init-tenant/template/fabric8-online-team-openshift.yml",
		"fabric8-online-team-openshift.yml",
	)
}

// generate_go reads file data from disk. It returns an error on failure.
func generate_go() ([]byte, error) {
	return bindata_read(
		"/home/aslak/.gvm/pkgsets/go1.7.3/almighty/src/github.com/fabric8io/fabric8-init-tenant/template/generate.go",
		"generate.go",
	)
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		return f()
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() ([]byte, error){
	"fabric8-online-che-openshift.yml": fabric8_online_che_openshift_yml,
	"fabric8-online-jenkins-openshift.yml": fabric8_online_jenkins_openshift_yml,
	"fabric8-online-project-openshift.yml": fabric8_online_project_openshift_yml,
	"fabric8-online-team-openshift.yml": fabric8_online_team_openshift_yml,
	"generate.go": generate_go,
}
// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for name := range node.Children {
		rv = append(rv, name)
	}
	return rv, nil
}

type _bintree_t struct {
	Func func() ([]byte, error)
	Children map[string]*_bintree_t
}
var _bintree = &_bintree_t{nil, map[string]*_bintree_t{
	"fabric8-online-che-openshift.yml": &_bintree_t{fabric8_online_che_openshift_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-jenkins-openshift.yml": &_bintree_t{fabric8_online_jenkins_openshift_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-project-openshift.yml": &_bintree_t{fabric8_online_project_openshift_yml, map[string]*_bintree_t{
	}},
	"fabric8-online-team-openshift.yml": &_bintree_t{fabric8_online_team_openshift_yml, map[string]*_bintree_t{
	}},
	"generate.go": &_bintree_t{generate_go, map[string]*_bintree_t{
	}},
}}
