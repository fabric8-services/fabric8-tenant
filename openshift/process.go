package openshift

import (
	"bytes"
	"html/template"
	"strings"
)

// Process takes a K8/Openshift Template as input and resolves the variable expresions
func Process(source string, variables map[string]string) (string, error) {
	target, err := template.New("openshift").Parse(replaceTemplateExpression(source))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = target.Execute(&buf, variables)
	if err != nil {
		return "", err
	}
	str := buf.String()
	return str, nil
}

func replaceTemplateExpression(template string) string {
	tmpl := template
	tmpl = strings.Replace(tmpl, "${", "{{.", -1)
	tmpl = strings.Replace(tmpl, "}", "}}", -1)

	return tmpl
}
