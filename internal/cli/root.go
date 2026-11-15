// Package cli builds DataWark's operator command line interface.
package cli

import (
	"context"
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
	"datawark/internal/postgres"
	"datawark/internal/principal"
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

	root := &cobra.Command{Use: "datawark", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newConfigCommand(deps), newSourceCommand(deps), newMCPCommand(deps), newServeCommand(deps, newHTTPProvided))
	return root
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
