// Package cli builds DataWark's operator command line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"datawark/internal/config"
	"datawark/internal/postgres"
	"github.com/spf13/cobra"
)

type Loader func(string) (config.Config, error)

type Pinger interface {
	Ping(context.Context, string) error
}

type Dependencies struct {
	Load   Loader
	Lookup func(string) (string, bool)
	Pinger Pinger
}

func NewRootCommand(deps Dependencies) *cobra.Command {
	if deps.Load == nil {
		deps.Load = config.Load
	}
	if deps.Lookup == nil {
		deps.Lookup = os.LookupEnv
	}
	if deps.Pinger == nil {
		deps.Pinger = postgres.Connector{}
	}

	root := &cobra.Command{Use: "datawark", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newConfigCommand(deps), newSourceCommand(deps))
	return root
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
