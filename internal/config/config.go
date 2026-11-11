// Package config loads and validates DataWark Gateway configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

const VersionV1Alpha1 = "v1alpha1"

type Config struct {
	Version     string       `yaml:"version"`
	Server      Server       `yaml:"server"`
	Datasources []Datasource `yaml:"datasources"`
	Policies    []Policy     `yaml:"policies"`
}

type Server struct {
	DevelopmentPrincipal string `yaml:"developmentPrincipal"`
}

type Datasource struct {
	Name   string `yaml:"name"`
	Driver string `yaml:"driver"`
	DSNEnv string `yaml:"dsnEnv"`
}

type Policy struct {
	Name       string       `yaml:"name"`
	Principals []string     `yaml:"principals"`
	Datasource string       `yaml:"datasource"`
	Allow      []AllowEntry `yaml:"allow"`
}

type AllowEntry struct {
	Table      string   `yaml:"table"`
	Operations []string `yaml:"operations"`
	Columns    []string `yaml:"columns"`
}

type Violation struct {
	Path    string
	Message string
}

type ValidationError struct {
	Violations []Violation
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, violation := range e.Violations {
		parts = append(parts, fmt.Sprintf("%s: %s", violation.Path, violation.Message))
	}
	return strings.Join(parts, "; ")
}

// Load reads, strictly decodes, and validates a configuration file. Decode
// failures are intentionally generalized so arbitrary YAML values are never
// reflected to callers or command output.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (Config, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return Config{}, errors.New("configuration: invalid YAML document")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return Config{}, errors.New("configuration: expected exactly one YAML document")
	}
	if violation := unknownFieldViolation(&document); violation != nil {
		return Config{}, &ValidationError{Violations: []Violation{*violation}}
	}
	var cfg Config
	if err := document.Decode(&cfg); err != nil {
		return Config{}, errors.New("configuration: invalid YAML document")
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func unknownFieldViolation(document *yaml.Node) *Violation {
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return &Violation{Path: "configuration", Message: "must be a mapping"}
	}
	return checkMapping(document.Content[0], "", map[string]map[string]struct{}{
		"":                   fieldSet("version", "server", "datasources", "policies"),
		"server":             fieldSet("developmentPrincipal"),
		"datasources[]":      fieldSet("name", "driver", "dsnEnv"),
		"policies[]":         fieldSet("name", "principals", "datasource", "allow"),
		"policies[].allow[]": fieldSet("table", "operations", "columns"),
	})
}

func fieldSet(fields ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		set[field] = struct{}{}
	}
	return set
}

func checkMapping(node *yaml.Node, path string, allowed map[string]map[string]struct{}) *Violation {
	key := mappingPath(path)
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		field := node.Content[i].Value
		value := node.Content[i+1]
		if _, ok := allowed[key][field]; !ok {
			return &Violation{Path: joinPath(path, field), Message: "is not supported"}
		}
		childPath := joinPath(path, field)
		switch childPath {
		case "server":
			if violation := checkMapping(value, childPath, allowed); violation != nil {
				return violation
			}
		case "datasources":
			for index, item := range value.Content {
				if violation := checkMapping(item, fmt.Sprintf("datasources[%d]", index), allowed); violation != nil {
					return violation
				}
			}
		case "policies":
			for index, item := range value.Content {
				if violation := checkMapping(item, fmt.Sprintf("policies[%d]", index), allowed); violation != nil {
					return violation
				}
			}
		}
		if strings.HasPrefix(path, "policies[") && field == "allow" {
			for index, item := range value.Content {
				if violation := checkMapping(item, fmt.Sprintf("%s.allow[%d]", path, index), allowed); violation != nil {
					return violation
				}
			}
		}
	}
	return nil
}

func mappingPath(path string) string {
	if strings.HasPrefix(path, "datasources[") {
		return "datasources[]"
	}
	if strings.HasPrefix(path, "policies[") && strings.Contains(path, ".allow[") {
		return "policies[].allow[]"
	}
	if strings.HasPrefix(path, "policies[") {
		return "policies[]"
	}
	return path
}

func joinPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}

func Validate(cfg Config) error {
	var violations []Violation
	add := func(path, message string) { violations = append(violations, Violation{Path: path, Message: message}) }

	if cfg.Version != VersionV1Alpha1 {
		add("version", "must be v1alpha1")
	}
	if strings.TrimSpace(cfg.Server.DevelopmentPrincipal) == "" {
		add("server.developmentPrincipal", "is required")
	}

	datasources := make(map[string]struct{}, len(cfg.Datasources))
	for i, datasource := range cfg.Datasources {
		prefix := fmt.Sprintf("datasources[%d]", i)
		name := strings.TrimSpace(datasource.Name)
		if name == "" {
			add(prefix+".name", "is required")
		} else if _, exists := datasources[name]; exists {
			add(prefix+".name", "must be unique")
		} else {
			datasources[name] = struct{}{}
		}
		if datasource.Driver != "postgres" {
			add(prefix+".driver", "must be postgres")
		}
		if strings.TrimSpace(datasource.DSNEnv) == "" {
			add(prefix+".dsnEnv", "is required")
		}
	}

	policies := make(map[string]struct{}, len(cfg.Policies))
	for i, policy := range cfg.Policies {
		prefix := fmt.Sprintf("policies[%d]", i)
		name := strings.TrimSpace(policy.Name)
		if name == "" {
			add(prefix+".name", "is required")
		} else if _, exists := policies[name]; exists {
			add(prefix+".name", "must be unique")
		} else {
			policies[name] = struct{}{}
		}
		if len(policy.Principals) == 0 {
			add(prefix+".principals", "must contain at least one principal")
		}
		for j, principal := range policy.Principals {
			if strings.TrimSpace(principal) == "" {
				add(fmt.Sprintf("%s.principals[%d]", prefix, j), "is required")
			}
		}
		if strings.TrimSpace(policy.Datasource) == "" {
			add(prefix+".datasource", "is required")
		} else if _, exists := datasources[policy.Datasource]; !exists {
			add(prefix+".datasource", "must reference a declared datasource")
		}
		if len(policy.Allow) == 0 {
			add(prefix+".allow", "must contain at least one entry")
		}
		for j, allow := range policy.Allow {
			allowPath := fmt.Sprintf("%s.allow[%d]", prefix, j)
			if strings.TrimSpace(allow.Table) == "" {
				add(allowPath+".table", "is required")
			}
			if len(allow.Operations) == 0 {
				add(allowPath+".operations", "must contain select")
			}
			for k, operation := range allow.Operations {
				if operation != "select" {
					add(fmt.Sprintf("%s.operations[%d]", allowPath, k), "must be select")
				}
			}
			if len(allow.Columns) == 0 {
				add(allowPath+".columns", "must contain at least one column")
			}
			for k, column := range allow.Columns {
				if strings.TrimSpace(column) == "" {
					add(fmt.Sprintf("%s.columns[%d]", allowPath, k), "is required")
				}
			}
		}
	}
	if len(violations) > 0 {
		return &ValidationError{Violations: violations}
	}
	return nil
}

func FindDatasource(cfg Config, name string) (Datasource, bool) {
	for _, datasource := range cfg.Datasources {
		if datasource.Name == name {
			return datasource, true
		}
	}
	return Datasource{}, false
}
