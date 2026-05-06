package cmd

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/lambdabaa/dewey/apps/cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	c.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print config file path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.ConfigPath()
			if err != nil {
				return err
			}
			cmd.Println(path)
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "get [key]",
		Short: "Print a config value (or all when omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(false)
			if err != nil {
				return err
			}
			cfg := app.Config
			r := app.Renderer
			if len(args) == 0 {
				if r.IsJSON() {
					return r.JSON(cfg)
				}
				r.KeyValue("default_collection", cfg.DefaultCollection)
				r.KeyValue("output", cfg.Output)
				r.KeyValue("color", cfg.Color)
				r.KeyValue("base_url", cfg.BaseURL)
				return nil
			}
			val := configValueByKey(cfg, args[0])
			if val == nil {
				return fmt.Errorf("unknown config key: %s", args[0])
			}
			cmd.Println(*val)
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := makeAppCtx(false)
			if err != nil {
				return err
			}
			cfg := app.Config
			if err := setConfigValue(cfg, args[0], args[1]); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			app.Renderer.Statusln(app.Renderer.Tick(), "set", args[0], "=", args[1])
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset CLI state (last-used collection, recent doc IDs)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := makeAppCtx(false)
			if err != nil {
				return err
			}
			app.State.LastCollection = ""
			app.State.RecentDocumentIDs = nil
			if err := app.State.Save(); err != nil {
				return err
			}
			app.Renderer.Statusln(app.Renderer.Tick(), "state cleared")
			return nil
		},
	})

	return c
}

var configKeys = []string{"default_collection", "output", "color", "base_url"}

func configValueByKey(cfg *config.Config, key string) *string {
	switch strings.ToLower(key) {
	case "default_collection":
		return &cfg.DefaultCollection
	case "output":
		return &cfg.Output
	case "color":
		return &cfg.Color
	case "base_url":
		return &cfg.BaseURL
	}
	return nil
}

func setConfigValue(cfg *config.Config, key, value string) error {
	v := reflect.ValueOf(cfg).Elem()
	switch strings.ToLower(key) {
	case "default_collection":
		v.FieldByName("DefaultCollection").SetString(value)
	case "output":
		if value != "human" && value != "json" {
			return fmt.Errorf("output must be 'human' or 'json'")
		}
		v.FieldByName("Output").SetString(value)
	case "color":
		if value != "auto" && value != "always" && value != "never" {
			return fmt.Errorf("color must be 'auto' | 'always' | 'never'")
		}
		v.FieldByName("Color").SetString(value)
	case "base_url":
		v.FieldByName("BaseURL").SetString(value)
	default:
		return fmt.Errorf("unknown config key %q (known: %s)", key, strings.Join(configKeys, ", "))
	}
	return nil
}
