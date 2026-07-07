package commands

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ctrwatch/internal/config"
	"ctrwatch/internal/runtime"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Name     string                    `yaml:"name"`
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	ContainerName string `yaml:"container_name"`
}

type kubeDoc struct {
	Spec kubeSpec `yaml:"spec"`
}

type kubeSpec struct {
	Containers  []namedContainer `yaml:"containers"`
	Template    *kubeTemplate    `yaml:"template"`
	JobTemplate *kubeJobTemplate `yaml:"jobTemplate"`
}

type kubeTemplate struct {
	Spec kubeSpec `yaml:"spec"`
}

type kubeJobTemplate struct {
	Spec struct {
		Template *kubeTemplate `yaml:"template"`
	} `yaml:"spec"`
}

type namedContainer struct {
	Name string `yaml:"name"`
}

// RunImport imports container names into the ctrwatch config.
func RunImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fromRunning := fs.Bool("from-running", false, "import currently running containers")
	tag := fs.String("tag", "dev", "tag for imported containers")
	output := fs.String("output", config.ConfigPath(), "config file to update")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var containers []string
	var err error
	if *fromRunning {
		containers, err = runningContainers()
	} else {
		containers, err = importContainers(fs.Arg(0))
	}
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return fmt.Errorf("import: no containers found")
	}
	*tag = strings.TrimSpace(*tag)
	if *tag == "" {
		return fmt.Errorf("import: tag is required")
	}

	cfg := &config.Config{}
	if existing, err := config.Load(*output); err == nil {
		cfg = existing
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	config.MergeServer(cfg, config.Server{
		Host:       "localhost",
		Containers: containers,
		Tags:       []string{*tag},
	})
	if err := config.Save(*output, cfg); err != nil {
		return err
	}
	fmt.Printf("imported %d containers into %s with tag @%s\n", len(containers), *output, *tag)
	return nil
}

func importContainers(path string) ([]string, error) {
	path, err := resolveImportPath(path)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}

	switch filepath.Ext(path) {
	case ".container", ".pod", ".kube":
		return quadletContainers(path, b)
	}

	if names, err := composeContainers(path, b); err == nil {
		return names, nil
	}
	return nil, fmt.Errorf("import: unsupported file %s", path)
}

func runningContainers() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containers, err := runtime.NewClient().ListContainers(ctx, false)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, containerName(c.Names))
	}
	return names, nil
}

func composeContainers(path string, b []byte) ([]string, error) {
	var compose composeFile
	if err := yaml.Unmarshal(b, &compose); err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	if len(compose.Services) == 0 {
		return nil, fmt.Errorf("import: no services in %s", path)
	}

	project := compose.Name
	if project == "" {
		project = filepath.Base(filepath.Dir(path))
	}

	names := make([]string, 0, len(compose.Services))
	for service, def := range compose.Services {
		if def.ContainerName != "" {
			names = append(names, def.ContainerName)
			continue
		}
		name := fmt.Sprintf("%s-%s-1", project, service)
		fmt.Fprintf(os.Stderr, "import: derived %s for service %s; set container_name for exact names\n", name, service)
		names = append(names, name)
	}
	return names, nil
}

func kubeContainers(b []byte) ([]string, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	var names []string
	for {
		var doc kubeDoc
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("import: %w", err)
		}
		names = appendKubeNames(names, doc.Spec, 0)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("import: no kube containers")
	}
	return names, nil
}

func appendKubeNames(names []string, spec kubeSpec, depth int) []string {
	if depth > 4 {
		return names
	}
	for _, c := range spec.Containers {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	if spec.Template != nil {
		names = appendKubeNames(names, spec.Template.Spec, depth+1)
	}
	if spec.JobTemplate != nil && spec.JobTemplate.Spec.Template != nil {
		names = appendKubeNames(names, spec.JobTemplate.Spec.Template.Spec, depth+1)
	}
	return names
}

func quadletContainers(path string, b []byte) ([]string, error) {
	for _, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		if key == "ContainerName" || key == "PodName" {
			return []string{strings.TrimSpace(value)}, nil
		}
		if key == "Yaml" {
			yamlPath := strings.TrimSpace(value)
			if !filepath.IsAbs(yamlPath) {
				yamlPath = filepath.Join(filepath.Dir(path), yamlPath)
			}
			kube, err := os.ReadFile(yamlPath)
			if err != nil {
				return nil, fmt.Errorf("import: %w", err)
			}
			return kubeContainers(kube)
		}
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name == "" {
		return nil, fmt.Errorf("import: no quadlet name in %s", path)
	}
	return []string{name}, nil
}

func resolveImportPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	for _, candidate := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("import: specify a Compose or Quadlet file")
}
