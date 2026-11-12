// Package runtime provides the immutable application boundary used by transports.
package runtime

import (
	"context"
	"datawark/internal/config"
	"datawark/internal/execution"
	"datawark/internal/policy"
	"datawark/internal/postgres"
	"datawark/internal/query"
	"datawark/internal/schema"
	"errors"
)

// Loader reads a fully validated Gateway configuration.
type Loader func(string) (config.Config, error)

// CatalogLoader loads the immutable metadata for one configured datasource.
type CatalogLoader interface {
	Load(context.Context, config.Config, string) (schema.Snapshot, error)
}

// Dependencies make bootstrap testable without exposing infrastructure to callers.
type Dependencies struct {
	LoadConfig Loader
	Catalog    CatalogLoader
	Executor   execution.Executor
}

// Runtime owns one configuration and one snapshot per configured datasource.
// Its fields remain private so callers cannot mutate its security inputs.
type Runtime struct {
	configuration config.Config
	snapshots     map[string]schema.Snapshot
	executor      execution.Executor
}

// New loads configuration from path and builds a complete runtime. It returns
// no runtime if any configuration, DSN, or catalog operation is unavailable.
func New(ctx context.Context, path string, dependencies Dependencies) (*Runtime, error) {
	load := dependencies.LoadConfig
	if load == nil {
		load = config.Load
	}
	configuration, err := load(path)
	if err != nil || config.Validate(configuration) != nil {
		return nil, errors.New("runtime: bootstrap failed")
	}
	return NewFromConfig(ctx, configuration, dependencies)
}

// NewFromConfig builds a complete runtime from a previously loaded config.
func NewFromConfig(ctx context.Context, configuration config.Config, dependencies Dependencies) (*Runtime, error) {
	if config.Validate(configuration) != nil {
		return nil, errors.New("runtime: bootstrap failed")
	}
	configuration = cloneConfig(configuration)
	catalog := dependencies.Catalog
	if catalog == nil {
		catalog = postgres.NewCatalogReader()
	}
	snapshots := make(map[string]schema.Snapshot, len(configuration.Datasources))
	for _, source := range configuration.Datasources {
		snapshot, err := catalog.Load(ctx, configuration, source.Name)
		if err != nil || snapshot.Datasource() != source.Name {
			return nil, errors.New("runtime: bootstrap failed")
		}
		snapshots[source.Name] = schema.NewSnapshot(snapshot.Datasource(), snapshot.Relations())
	}
	if dependencies.Executor == nil {
		dependencies.Executor = postgres.NewExecutor()
	}
	return &Runtime{configuration: configuration, snapshots: snapshots, executor: dependencies.Executor}, nil
}

// Execute submits a plan to the existing complete query security pipeline.
func (runtime *Runtime) Execute(ctx context.Context, plan query.Plan) (execution.Result, error) {
	snapshot, found := runtime.snapshot(plan.Source)
	if !found {
		return execution.Result{}, errors.New("runtime: datasource is unavailable")
	}
	service := execution.QueryService{Configuration: cloneConfig(runtime.configuration), Snapshot: snapshot, Executor: runtime.executor}
	return service.Execute(ctx, plan)
}

// Datasources lists only sources with metadata visible to the configured principal.
func (runtime *Runtime) Datasources() []string {
	visible := make([]string, 0)
	for _, source := range runtime.configuration.Datasources {
		snapshot, _ := runtime.snapshot(source.Name)
		filtered, err := schema.Visible(runtime.configuration.Server.DevelopmentPrincipal, &runtime.configuration, &snapshot)
		if err == nil && len(filtered.Relations()) > 0 {
			visible = append(visible, source.Name)
		}
	}
	return visible
}

// Schema returns only relations and columns visible to the configured principal.
func (runtime *Runtime) Schema(datasource string) (schema.Snapshot, error) {
	snapshot, found := runtime.snapshot(datasource)
	if !found {
		return schema.Snapshot{}, errors.New("runtime: datasource is unavailable")
	}
	return schema.Visible(runtime.configuration.Server.DevelopmentPrincipal, &runtime.configuration, &snapshot)
}

// Explain applies grammar, policy, and metadata gates without compiling or executing SQL.
func (runtime *Runtime) Explain(plan query.Plan) policy.Explanation {
	validated, err := query.Validate(plan)
	if err != nil {
		return policy.Explanation{Reason: policy.ReasonDenied}
	}
	explanation := policy.Explain(policy.Input{Principal: runtime.configuration.Server.DevelopmentPrincipal, Snapshot: &runtime.configuration, Plan: validated})
	if !explanation.Allowed {
		return explanation
	}
	snapshot, found := runtime.snapshot(plan.Source)
	if !found || schema.ValidatePlan(validated, &snapshot) != nil {
		return policy.Explanation{Reason: policy.ReasonDenied}
	}
	return explanation
}

func (runtime *Runtime) snapshot(datasource string) (schema.Snapshot, bool) {
	if runtime == nil {
		return schema.Snapshot{}, false
	}
	snapshot, found := runtime.snapshots[datasource]
	if !found {
		return schema.Snapshot{}, false
	}
	return schema.NewSnapshot(snapshot.Datasource(), snapshot.Relations()), true
}

func cloneConfig(configuration config.Config) config.Config {
	copy := configuration
	copy.Datasources = append([]config.Datasource(nil), configuration.Datasources...)
	copy.Policies = make([]config.Policy, len(configuration.Policies))
	for i, policy := range configuration.Policies {
		copy.Policies[i] = policy
		copy.Policies[i].Principals = append([]string(nil), policy.Principals...)
		copy.Policies[i].Allow = make([]config.AllowEntry, len(policy.Allow))
		for j, allow := range policy.Allow {
			copy.Policies[i].Allow[j] = allow
			copy.Policies[i].Allow[j].Operations = append([]string(nil), allow.Operations...)
			copy.Policies[i].Allow[j].Columns = append([]string(nil), allow.Columns...)
		}
	}
	if configuration.Server.Runtime != nil {
		runtime := *configuration.Server.Runtime
		copy.Server.Runtime = &runtime
	}
	return copy
}
