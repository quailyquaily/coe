package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

const (
	TemplateASRDefault            = "asr-default.tmpl"
	TemplateLLMCorrection         = "llm-correction.tmpl"
	TemplateLLMCorrectionGeneral  = "llm-correction-general.tmpl"
	TemplateLLMCorrectionTerminal = "llm-correction-terminal.tmpl"
	TemplateSceneRouter           = "scene-router.tmpl"
)

type ASRTemplateData struct {
	Provider string
	Model    string
	Language string
}

type LLMTemplateData struct {
	Provider     string
	Model        string
	EndpointType string
}

//go:embed templates/*.tmpl
var templateFiles embed.FS

var (
	loadTemplatesOnce  sync.Once
	loadedTemplates    *template.Template
	loadedTemplatesErr error
)

func ResolveASR(override, overrideFile string, data ASRTemplateData) (string, error) {
	return ResolveNamed(TemplateASRDefault, override, overrideFile, data)
}

func ResolveLLMCorrection(override, overrideFile string, data LLMTemplateData) (string, error) {
	return ResolveNamed(TemplateLLMCorrection, override, overrideFile, data)
}

func ResolveLLMCorrectionGeneral(override, overrideFile string, data LLMTemplateData) (string, error) {
	return ResolveNamed(TemplateLLMCorrectionGeneral, override, overrideFile, data)
}

func ResolveLLMCorrectionTerminal(override, overrideFile string, data LLMTemplateData) (string, error) {
	return ResolveNamed(TemplateLLMCorrectionTerminal, override, overrideFile, data)
}

func ResolveSceneRouter(data LLMTemplateData) (string, error) {
	return ResolveNamed(TemplateSceneRouter, "", "", data)
}

func ResolveNamed(defaultName, override, overrideFile string, data any) (string, error) {
	if strings.TrimSpace(overrideFile) != "" {
		return executeFile(overrideFile, data)
	}
	if strings.TrimSpace(override) != "" {
		return executeInline(defaultName+" override", override, data)
	}
	return executeNamed(defaultName, data)
}

func executeNamed(name string, data any) (string, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render prompt template %s: %w", name, err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func executeInline(name, source string, data any) (string, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse prompt template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt template %s: %w", name, err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func executeFile(path string, data any) (string, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read prompt template %s: %w", path, err)
	}

	tmpl, err := template.New(filepath.Base(path)).Option("missingkey=error").Parse(string(source))
	if err != nil {
		return "", fmt.Errorf("parse prompt template %s: %w", path, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt template %s: %w", path, err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func loadTemplates() (*template.Template, error) {
	loadTemplatesOnce.Do(func() {
		loadedTemplates, loadedTemplatesErr = template.New("prompts").Option("missingkey=error").ParseFS(templateFiles, "templates/*.tmpl")
	})
	return loadedTemplates, loadedTemplatesErr
}
