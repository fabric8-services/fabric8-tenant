package environment

import (
	"context"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/environment/generated"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	"github.com/fabric8-services/fabric8-tenant/utils"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"path"
	"strconv"
	"strings"
	"time"
)

//go:generate go-bindata -prefix "./templates/" -pkg templates -o ./generated/templates.go ./templates/...

const (
	f8TenantServiceRepoUrl = "https://github.com/fabric8-services/fabric8-tenant"
	rawFileURLTemplate     = "%s/blob/%s/%s"
	templatesDirectory     = "environment/templates/"
)

var DefaultEnvTypes = []string{"che", "jenkins", "user", "run", "stage"}

var templateNames = map[string][]*Template{
	"run":     tmpls(runParams, "fabric8-tenant-deploy.yml"),
	"stage":   tmpls(stageParams, "fabric8-tenant-deploy.yml"),
	"che-mt":  tmpls(noParams, "fabric8-tenant-che-mt.yml", "fabric8-tenant-che-quotas.yml"),
	"che":     tmpls(noParams, "fabric8-tenant-che.yml", "fabric8-tenant-che-quotas.yml"),
	"jenkins": tmpls(noParams, "fabric8-tenant-jenkins.yml", "fabric8-tenant-jenkins-quotas.yml"),
	"user":    tmpls(noParams, "fabric8-tenant-user.yml"),
}

func tmpls(defaultParams map[string]string, filenames ...string) []*Template {
	tmpls := make([]*Template, 0, len(filenames))
	for _, fileName := range filenames {
		tmpls = append(tmpls, newTemplate(fileName, defaultParams))
	}
	return tmpls
}

type Service struct {
	templatesRepo     string
	templatesRepoBlob string
	templatesRepoDir  string
}

func NewService(templatesRepo, templatesRepoBlob, templatesRepoDir string) *Service {
	return &Service{
		templatesRepo:     templatesRepo,
		templatesRepoBlob: templatesRepoBlob,
		templatesRepoDir:  templatesRepoDir,
	}
}

type EnvData struct {
	NsType    string
	Name      string
	Templates []*Template
}

func (s *Service) GetEnvData(ctx context.Context, envType string) (*EnvData, error) {
	var templates []*Template
	if envType == "che" {
		if toggles.IsEnabled(ctx, "deploy.che-multi-tenant", false) {
			token := goajwt.ContextJWT(ctx)
			var cheMtParams map[string]string
			if token != nil {
				cheMtParams["OSIO_TOKEN"] = token.Raw
				id := token.Claims.(jwt.MapClaims)["sub"]
				if id == nil {
					return nil, errors.New("missing sub in JWT token")
				}
				cheMtParams["IDENTITY_ID"] = id.(string)
			}
			cheMtParams["REQUEST_ID"] = log.ExtractRequestID(ctx)
			unixNano := time.Now().UnixNano()
			cheMtParams["JOB_ID"] = strconv.FormatInt(unixNano/1000000, 10)

			templates = templateNames["che-mt"]
			templates[0].DefaultParams = cheMtParams
		} else {
			templates = templateNames[envType]
		}
	} else {
		templates = templateNames[envType]
	}

	err := s.retrieveTemplates(templates)
	if err != nil {
		return nil, err
	}

	return &EnvData{
		Templates: templates,
		Name:      envType,
		NsType:    envType,
	}, nil

}

func (s *Service) retrieveTemplates(tmpls []*Template) error {
	var (
		content []byte
		err     error
	)
	for _, template := range tmpls {
		if s.templatesRepoBlob != "" {
			fileURL := fmt.Sprintf(rawFileURLTemplate, s.getRepo(), s.templatesRepoBlob, s.getPath(template))
			content, err = utils.DownloadFile(fileURL)
		} else {
			content, err = templates.Asset(template.Filename)
		}
		if err != nil {
			return err
		}
		template.Content = string(content)
	}
	return nil
}

func (s *Service) getRepo() string {
	return get(s.templatesRepo, f8TenantServiceRepoUrl)
}

func (s *Service) getPath(template *Template) string {
	return path.Clean(get(s.templatesRepoDir, templatesDirectory) + "/" + template.Filename)
}

func get(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
