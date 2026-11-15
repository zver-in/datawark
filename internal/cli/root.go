// Package cli builds DataWark's operator command line interface.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"datawark/internal/audit"
	"datawark/internal/config"
	"datawark/internal/httpserver"
	"datawark/internal/mcpserver"
	"datawark/internal/policy"
	"datawark/internal/postgres"
	"datawark/internal/principal"
	"datawark/internal/query"
	"datawark/internal/runtime"
	"github.com/spf13/cobra"
)

type Loader func(string) (config.Config, error)

type Pinger interface {
	Ping(context.Context, string) error
}

type Bootstrap func(context.Context, string) (*runtime.Runtime, config.Config, error)
type ListenerFactory func(string, string) (net.Listener, error)

type Dependencies struct {
	Load      Loader
	Lookup    func(string) (string, bool)
	Pinger    Pinger
	Runtime   func(context.Context, string) (*runtime.Runtime, error)
	RunMCP    func(context.Context, runtime.Gateway) error
	Bootstrap Bootstrap
	Listen    ListenerFactory
	NewHTTP   func(string, *httpserver.Authenticator) *httpserver.Server
	NewAuth   func([]config.APIKey, func(string) (string, bool)) (*httpserver.Authenticator, error)
	Open      func(string) (io.ReadCloser, error)
}

const maxPlanBytes int64 = 1 << 20

type planExplanation struct {
	Version  string `json:"version"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
	PolicyID string `json:"policyId,omitempty"`
}

func NewRootCommand(deps Dependencies) *cobra.Command {
	newHTTPProvided := deps.NewHTTP != nil
	if deps.Load == nil {
		deps.Load = config.Load
	}
	if deps.Lookup == nil {
		deps.Lookup = os.LookupEnv
	}
	if deps.Pinger == nil {
		deps.Pinger = postgres.Connector{}
	}
	if deps.Runtime == nil {
		deps.Runtime = func(ctx context.Context, path string) (*runtime.Runtime, error) {
			return runtime.New(ctx, path, runtime.Dependencies{})
		}
	}
	if deps.RunMCP == nil {
		deps.RunMCP = mcpserver.Run
	}
	if deps.Bootstrap == nil {
		deps.Bootstrap = func(ctx context.Context, path string) (*runtime.Runtime, config.Config, error) {
			configuration, err := deps.Load(path)
			if err != nil {
				return nil, config.Config{}, err
			}
			gateway, err := runtime.NewFromConfig(ctx, configuration, runtime.Dependencies{})
			return gateway, configuration, err
		}
	}
	if deps.Listen == nil {
		deps.Listen = net.Listen
	}
	if deps.NewHTTP == nil {
		deps.NewHTTP = func(address string, authenticator *httpserver.Authenticator) *httpserver.Server {
			return httpserver.New(address, authenticator)
		}
	}
	if deps.NewAuth == nil {
		deps.NewAuth = httpserver.NewAuthenticator
	}
	if deps.Open == nil {
		deps.Open = func(path string) (io.ReadCloser, error) { return os.Open(path) }
	}

	root := &cobra.Command{Use: "datawark", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newConfigCommand(deps), newSourceCommand(deps), newMCPCommand(deps), newPolicyCommand(deps), newServeCommand(deps, newHTTPProvided))
	return root
}

func newPolicyCommand(deps Dependencies) *cobra.Command {
	policyCommand := &cobra.Command{Use: "policy"}
	var path, planSource string
	explain := &cobra.Command{
		Use: "explain",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := readPlan(cmd.InOrStdin(), deps.Open, planSource)
			if err != nil {
				return errors.New("query plan input is invalid")
			}
			runtime, err := deps.Runtime(cmd.Context(), path)
			if err != nil || runtime == nil {
				return errors.New("policy runtime bootstrap failed")
			}
			gateway, err := runtime.LocalGateway()
			if err != nil || gateway == nil {
				return errors.New("policy runtime bootstrap failed")
			}
			explanation := gateway.Explain(plan.Plan)
			result := explanationDTO(explanation)
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
				return errors.New("policy explanation output failed")
			}
			return nil
		},
	}
	explain.Flags().StringVar(&path, "config", "", "path to configuration YAML")
	explain.Flags().StringVar(&planSource, "plan", "", "path to canonical query plan JSON, or - for standard input")
	_ = explain.MarkFlagRequired("config")
	_ = explain.MarkFlagRequired("plan")
	policyCommand.AddCommand(explain)
	return policyCommand
}

func readPlan(stdin io.Reader, open func(string) (io.ReadCloser, error), source string) (query.ValidatedPlan, error) {
	if source == "" {
		return query.ValidatedPlan{}, errors.New("plan source is required")
	}
	reader := stdin
	var file io.ReadCloser
	if source != "-" {
		var err error
		file, err = open(source)
		if err != nil {
			return query.ValidatedPlan{}, errors.New("plan source unavailable")
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxPlanBytes+1))
	if err != nil || int64(len(data)) > maxPlanBytes {
		return query.ValidatedPlan{}, errors.New("plan source invalid")
	}
	return query.Decode(data)
}

func explanationDTO(explanation policy.Explanation) planExplanation {
	result := planExplanation{Version: "v1alpha1", Allowed: explanation.Allowed, Reason: string(explanation.Reason)}
	if explanation.Allowed {
		result.PolicyID = explanation.PolicyID
	}
	return result
}

func newServeCommand(deps Dependencies, newHTTPProvided bool) *cobra.Command {
	var path string
	serve := &cobra.Command{
		Use: "serve",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			gateway, configuration, err := deps.Bootstrap(ctx, path)
			if err != nil || gateway == nil {
				return errors.New("HTTP runtime bootstrap failed")
			}
			authenticator, err := deps.NewAuth(configuration.Server.HTTPListener().APIKeys, deps.Lookup)
			if err != nil || authenticator == nil {
				return errors.New("HTTP authentication bootstrap failed")
			}
			authenticator.SetRecorder(audit.NewJSONLinesRecorder(cmd.OutOrStdout()))
			listener, err := deps.Listen("tcp", configuration.Server.HTTPListener().ListenAddress)
			if err != nil {
				return errors.New("HTTP listener could not be started")
			}
			service := deps.NewHTTP(configuration.Server.HTTPListener().ListenAddress, authenticator)
			if !newHTTPProvided {
				service = httpserver.NewWithGateway(configuration.Server.HTTPListener().ListenAddress, authenticator, func(identity principal.Trusted) (httpserver.Gateway, error) {
					return gateway.Bind(identity)
				})
			}
			serveErrors := make(chan error, 1)
			go func() { serveErrors <- service.Serve(listener) }()
			select {
			case <-ctx.Done():
				shutdown, cancel := context.WithTimeout(context.Background(), httpserver.ShutdownTimeout)
				defer cancel()
				if err := service.Shutdown(shutdown); err != nil {
					return errors.New("HTTP server shutdown failed")
				}
				return <-serveErrors
			case err := <-serveErrors:
				if err != nil {
					return errors.New("HTTP server failed")
				}
				return nil
			}
		},
	}
	serve.Flags().StringVar(&path, "config", "", "path to configuration YAML")
	_ = serve.MarkFlagRequired("config")
	return serve
}

func newMCPCommand(deps Dependencies) *cobra.Command {
	mcp := &cobra.Command{Use: "mcp"}
	var path string
	stdio := &cobra.Command{
		Use: "stdio",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runtime, err := deps.Runtime(cmd.Context(), path)
			if err != nil {
				return errors.New("MCP runtime bootstrap failed")
			}
			gateway, err := runtime.LocalGateway()
			if err != nil {
				return errors.New("MCP runtime bootstrap failed")
			}
			return deps.RunMCP(cmd.Context(), gateway)
		},
	}
	stdio.Flags().StringVar(&path, "config", "", "path to configuration YAML")
	_ = stdio.MarkFlagRequired("config")
	mcp.AddCommand(stdio)
	return mcp
}

func newConfigCommand(deps Dependencies) *cobra.Command {
	configCommand := &cobra.Command{Use: "config"}
	var path string
	validate := &cobra.Command{
		Use: "validate",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := deps.Load(path); err != nil {
				return safeConfigError(err)
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "configuration is valid")
			return err
		},
	}
	validate.Flags().StringVar(&path, "config", "", "path to configuration YAML")
	_ = validate.MarkFlagRequired("config")
	configCommand.AddCommand(validate)
	return configCommand
}

func newSourceCommand(deps Dependencies) *cobra.Command {
	source := &cobra.Command{Use: "source"}
	var path, datasource string
	test := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(path)
			if err != nil {
				return safeConfigError(err)
			}
			source, found := config.FindDatasource(cfg, datasource)
			if !found {
				return fmt.Errorf("datasource %q is not declared", datasource)
			}
			dsn, found := deps.Lookup(source.DSNEnv)
			if !found || strings.TrimSpace(dsn) == "" {
				return fmt.Errorf("environment variable %q is not set", source.DSNEnv)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			if err := deps.Pinger.Ping(ctx, dsn); err != nil {
				return errors.New("PostgreSQL connectivity check failed")
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "datasource %q is reachable\n", datasource)
			return err
		},
	}
	test.Flags().StringVar(&path, "config", "", "path to configuration YAML")
	test.Flags().StringVar(&datasource, "datasource", "", "datasource name")
	_ = test.MarkFlagRequired("config")
	_ = test.MarkFlagRequired("datasource")
	source.AddCommand(test)
	return source
}

func safeConfigError(err error) error {
	var validation *config.ValidationError
	if errors.As(err, &validation) {
		return validation
	}
	return errors.New("configuration could not be loaded")
}

func Execute(args []string, stdout, stderr io.Writer) int {
	command := NewRootCommand(Dependencies{})
	command.SetArgs(args)
	command.SetOut(stdout)
	command.SetErr(stderr)
	if err := command.Execute(); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
