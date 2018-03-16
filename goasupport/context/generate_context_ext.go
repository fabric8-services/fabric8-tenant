package context

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goadesign/goa/design"
	"github.com/goadesign/goa/goagen/codegen"
)

// Generate adds method to support conditional queries
func Generate() ([]string, error) {
	fmt.Println("Generating extensions for goa contexts...")
	var (
		ver    string
		outDir string
	)
	set := flag.NewFlagSet("app", flag.PanicOnError)
	set.String("design", "", "") // Consume design argument so Parse doesn't complain
	set.StringVar(&ver, "version", "", "")
	set.StringVar(&outDir, "out", "", "")
	set.Parse(os.Args[2:])

	// First check compatibility
	if err := codegen.CheckVersion(ver); err != nil {
		return nil, err
	}

	return writeContextExtensions(design.Design, outDir)
}

// WriteNames creates the names.txt file.
func writeContextExtensions(api *design.APIDefinition, outDir string) ([]string, error) {
	ctxFile := filepath.Join(outDir, "context_extensions.go")
	ctxWr, err := codegen.SourceFileFor(ctxFile)
	if err != nil {
		panic(err) // bug
	}

	title := fmt.Sprintf("%s: GOA Context extensions - See goasupport/context/generator.go", api.Context())
	imports := []*codegen.ImportSpec{
		codegen.SimpleImport("net/http"),
		codegen.SimpleImport("fmt"),
		codegen.SimpleImport("context"),
		codegen.SimpleImport("strings"),
	}

	ctxWr.WriteHeader(title, "app", imports)
	if err := ctxWr.ExecuteTemplate("absoluteURL", absoluteURL, nil, nil); err != nil {
		return nil, err
	}

	// Now iterate through the resources to gather their names
	api.IterateResources(func(res *design.ResourceDefinition) error {
		res.IterateActions(func(act *design.ActionDefinition) error {
			// by default, do not support the `WithForwardPath` option when computing a relative URL
			tmplSource := defaultAbsoluteURL
			act.IterateHeaders(func(name string, isRequired bool, h *design.AttributeDefinition) error {
				if name == "X-Forwarded-Path" {
					tmplSource = absoluteURLWithForwardPath
				}
				return nil
			})

			ctxName := fmt.Sprintf("%v%vContext", codegen.Goify(act.Name, true), codegen.Goify(res.Name, true))
			fmt.Printf("writing extension for %s...\n", ctxName)

			err := ctxWr.ExecuteTemplate("relativeURL extension", tmplSource, nil, struct {
				Name string
			}{
				Name: ctxName,
			})
			return err
		})
		return nil
	})

	err = ctxWr.FormatCode()
	if err != nil {
		return nil, err
	}
	return []string{ctxFile}, nil
}

const (
	absoluteURL = `

type relativeURLOption func(string) string

func WithXForwardPath(baseURL string, forwardPath *string) relativeURLOption {
	return func(currentURL string) string {
		if forwardPath == nil {
			return currentURL
		}
		return strings.Replace(currentURL, baseURL, *forwardPath, 1)

	}
}

type AbsoluteURL interface {
	context.Context
	AbsoluteURL(relativeURL string) string
}

// absoluteURL prefixes a relative URL with absolute address
func absoluteURL(req *http.Request, relative string, opts ...relativeURLOption) string {
	scheme := "http"
	if req.URL != nil && req.URL.Scheme == "https" { // isHTTPS
		scheme = "https"
	}
	xForwardProto := req.Header.Get("X-Forwarded-Proto")
	if xForwardProto != "" {
		scheme = xForwardProto
	}
	for _, opt := range opts {
		relative = opt(relative)
	}
	return fmt.Sprintf("%s://%s%s", scheme, req.Host, relative)
}

`

	defaultAbsoluteURL = `func (ctx *{{ .Name }}) AbsoluteURL(relativeURL string) string {
	return absoluteURL(ctx.Request, relativeURL)
}

`
	absoluteURLWithForwardPath = `func (ctx *{{ .Name }}) AbsoluteURL(relativeURL string) string {
	return absoluteURL(ctx.Request, relativeURL, WithXForwardPath(TenantHref(), ctx.XForwardedPath))
}

`
)
