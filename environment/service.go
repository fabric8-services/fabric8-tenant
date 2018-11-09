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
	f8TenantServiceRepoUrl = "https://raw.githubusercontent.com/fabric8-services/fabric8-tenant"
	rawFileURLTemplate     = "%s/%s/%s"
	templatesDirectory     = "environment/templates/"
)

var (
	VersionFabric8TenantUserFile          string
	VersionFabric8TenantCheMtFile         string
	VersionFabric8TenantJenkinsFile       string
	VersionFabric8TenantJenkinsQuotasFile string
	VersionFabric8TenantCheFile           string
	VersionFabric8TenantCheQuotasFile     string
	VersionFabric8TenantDeployFile        string
	DefaultEnvTypes                       = []string{"che", "jenkins", "user", "run", "stage"}
)

type Templates []*Template

func RetrieveMappedTemplates() map[string]Templates {
	return map[string]Templates{
		"run":   tmpl(deploy("run"), "fabric8-tenant-deploy.yml"),
		"stage": tmpl(deploy("stage"), "fabric8-tenant-deploy.yml"),
		"che-mt": tmplWithQuota(versions(VersionFabric8TenantCheMtFile, VersionFabric8TenantCheQuotasFile),
			"fabric8-tenant-che-mt.yml", "fabric8-tenant-che-quotas.yml"),
		"che": tmplWithQuota(versions(VersionFabric8TenantCheFile, VersionFabric8TenantCheQuotasFile),
			"fabric8-tenant-che.yml", "fabric8-tenant-che-quotas.yml"),
		"jenkins": tmplWithQuota(versions(VersionFabric8TenantJenkinsFile, VersionFabric8TenantJenkinsQuotasFile),
			"fabric8-tenant-jenkins.yml", "fabric8-tenant-jenkins-quotas.yml"),
		"user": tmpl(versions(VersionFabric8TenantUserFile, ""), "fabric8-tenant-user.yml"),
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
	return []*Template{newTemplate(fileName, defaultParams, defaultParams[varCommit]),
		newTemplate(quotasFileName, defaultParams, defaultParams[varCommitQuotas])}
}

func tmpl(defaultParams map[string]string, fileName string) []*Template {
	return []*Template{newTemplate(fileName, defaultParams, defaultParams[varCommit])}
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
	Templates Templates
}

func (s *Service) GetEnvData(ctx context.Context, envType string) (*EnvData, error) {
	var templates []*Template
	var mappedTemplates = RetrieveMappedTemplates()
	if envType == "che" {
		if ctx != nil && !toggles.IsEnabled(ctx, "deploy.che-multi-tenant", false) {
			templates = mappedTemplates[envType]
		} else {
			cheMtParams, err := getCheMtParams(ctx)
			if err != nil {
				return nil, err
			}
			templates = mappedTemplates["che-mt"]
			templates[0].DefaultParams = merge(templates[0].DefaultParams, cheMtParams, true)
		}
	} else {
		templates = mappedTemplates[envType]
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

func getCheMtParams(ctx context.Context) (map[string]string, error) {
	var cheMtParams = map[string]string{}
	if ctx != nil {
		token := goajwt.ContextJWT(ctx)
		if token != nil {
			cheMtParams["OSIO_TOKEN"] = token.Raw
			id := token.Claims.(jwt.MapClaims)["sub"]
			if id == nil {
				return nil, errors.New("missing sub in JWT token")
			}
			cheMtParams["IDENTITY_ID"] = id.(string)
		}
		cheMtParams["REQUEST_ID"] = log.ExtractRequestID(ctx)
	}
	unixNano := time.Now().UnixNano()
	cheMtParams["JOB_ID"] = strconv.FormatInt(unixNano/1000000, 10)

	return cheMtParams, nil
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
