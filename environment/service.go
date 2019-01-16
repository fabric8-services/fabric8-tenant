package environment

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-common/log"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/environment/generated"
	"github.com/fabric8-services/fabric8-tenant/utils"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"path"
	"strconv"
	"strings"
	"time"
)

//go:generate go-bindata -prefix "./templates/" -pkg templates -o ./generated/templates.go ./templates/...

const (
	f8TenantServiceRepoUrl = "https://raw.githubusercontent.com/fabric8-services/fabric8-tenant"
	rawFileURLTemplate     = "%s/%s/%s"
	templatesDirectory     = "environment/templates/"
)

var (
	VersionFabric8TenantUserFile          string
	VersionFabric8TenantCheMtFile         string
	VersionFabric8TenantJenkinsFile       string
	VersionFabric8TenantJenkinsQuotasFile string
	VersionFabric8TenantCheQuotasFile     string
	VersionFabric8TenantDeployFile        string
	DefaultEnvTypes                       = []Type{TypeChe, TypeJenkins, TypeRun, TypeStage, TypeUser}
)

type Templates []*Template

func RetrieveMappedTemplates() map[Type]Templates {
	return map[Type]Templates{
		TypeRun:   tmpl(deploy("run"), "fabric8-tenant-deploy.yml"),
		TypeStage: tmpl(deploy("stage"), "fabric8-tenant-deploy.yml"),
		TypeChe: tmplWithQuota(versions(VersionFabric8TenantCheMtFile, VersionFabric8TenantCheQuotasFile),
			"fabric8-tenant-che-mt.yml", "fabric8-tenant-che-quotas.yml"),
		TypeJenkins: tmplWithQuota(versions(VersionFabric8TenantJenkinsFile, VersionFabric8TenantJenkinsQuotasFile),
			"fabric8-tenant-jenkins.yml", "fabric8-tenant-jenkins-quotas.yml"),
		TypeUser: tmpl(versions(VersionFabric8TenantUserFile, ""), "fabric8-tenant-user.yml"),
	}
}

func versions(version, quotasVersion string) map[string]string {
	return map[string]string{varCommit: version, varCommitQuotas: quotasVersion}
}

func deploy(stage string) map[string]string {
	return map[string]string{
		varCommit:     VersionFabric8TenantDeployFile,
		varDeployType: stage,
	}
}

func tmplWithQuota(defaultParams map[string]string, fileName string, quotasFileName string) []*Template {
	tmpl := newTemplate(fileName, defaultParams, defaultParams[varCommit])
	quotas := newTemplate(quotasFileName, defaultParams, defaultParams[varCommitQuotas])
	return []*Template{&tmpl, &quotas}
}

func tmpl(defaultParams map[string]string, fileName string) []*Template {
	tmpl := newTemplate(fileName, defaultParams, defaultParams[varCommit])
	return []*Template{&tmpl}
}

func (t Templates) ConstructCompleteVersion() string {
	var versions []string
	for _, template := range t {
		versions = append(versions, template.Version)
	}
	return strings.Join(versions, "_")
}

type Service struct {
	templatesRepo     string
	templatesRepoBlob string
	templatesRepoDir  string
}

func NewService() *Service {
	return &Service{}
}

func NewServiceForUserData(user *authclient.UserDataAttributes) *Service {
	service := NewService()
	if user != nil {
		if user.FeatureLevel != nil && *user.FeatureLevel != "internal" {
			return service
		}
		userContext := user.ContextInformation
		if tc, found := userContext["tenantConfig"]; found {
			if tenantConfig, ok := tc.(map[string]interface{}); ok {
				find := func(key string) string {
					if rawValue, found := tenantConfig[key]; found {
						if value, ok := rawValue.(string); ok {
							return value
						}
					}
					return ""
				}
				service.templatesRepo = find("templatesRepo")
				service.templatesRepoBlob = find("templatesRepoBlob")
				service.templatesRepoDir = find("templatesRepoDir")
			}
		}
	}
	return service
}

type EnvData struct {
	EnvType   Type
	Templates Templates
}

func (e *EnvData) Version() string {
	return e.Templates.ConstructCompleteVersion()
}

func (s *Service) GetEnvData(ctx context.Context, envType Type) (*EnvData, error) {
	var templates Templates
	var mappedTemplates = RetrieveMappedTemplates()
	templates = mappedTemplates[envType]

	if envType == TypeChe {
		err := getCheParams(ctx, templates[0].DefaultParams)
		if err != nil {
			return nil, err
		}
	}

	err := s.retrieveTemplates(templates)
	if err != nil {
		return nil, err
	}

	return &EnvData{
		Templates: templates,
		EnvType:   envType,
	}, nil
}

func getCheParams(ctx context.Context, defaultParams map[string]string) error {
	if ctx != nil {
		token := goajwt.ContextJWT(ctx)
		if token != nil {
			defaultParams["OSIO_TOKEN"] = token.Raw
			id := token.Claims.(jwt.MapClaims)["sub"]
			if id != nil {
				defaultParams["IDENTITY_ID"] = id.(string)
			}
		}
		defaultParams["REQUEST_ID"] = log.ExtractRequestID(ctx)
	}
	unixNano := time.Now().UnixNano()
	defaultParams["JOB_ID"] = strconv.FormatInt(unixNano/1000000, 10)
	return nil
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
			template.DefaultParams[varCommit] = s.templatesRepoBlob
			template.DefaultParams[varCommitQuotas] = s.templatesRepoBlob
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
	repo := strings.TrimSpace(s.templatesRepo)
	if repo == "" {
		return f8TenantServiceRepoUrl
	}
	if strings.Contains(repo, "github.com") {
		return strings.Replace(repo, "github.com", "raw.githubusercontent.com", 1)
	}
	return repo
}

func (s *Service) getPath(template *Template) string {
	directory := strings.TrimSpace(s.templatesRepoDir)
	if directory == "" {
		directory = templatesDirectory
	}
	return path.Clean(directory + "/" + template.Filename)
}
