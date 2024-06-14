package ecspresso

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/fujiwara/cfn-lookup/cfn"
	"github.com/fujiwara/ssm-lookup/ssm"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/google/go-jsonnet"
	"github.com/kayac/ecspresso/v2/secretsmanager"
	"github.com/samber/lo"
)

var defaultPluginNames = []string{"ssm", "secretsmanager"}

type ConfigPlugin struct {
	Name       string                 `yaml:"name" json:"name,omitempty"`
	Config     map[string]interface{} `yaml:"config" json:"config,omitempty"`
	FuncPrefix string                 `yaml:"func_prefix,omitempty" json:"func_prefix,omitempty"`
}

func (p ConfigPlugin) Setup(ctx context.Context, c *Config) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(ctx, p, c)
	case "cloudformation":
		return setupPluginCFn(ctx, p, c)
	case "ssm":
		return setupPluginSSM(ctx, p, c)
	case "secretsmanager":
		return setupPluginSecretsManager(ctx, p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func (p ConfigPlugin) AppendFuncMap(c *Config, funcMap template.FuncMap) error {
	modified := make(template.FuncMap, len(funcMap))
	for funcName, f := range funcMap {
		name := p.FuncPrefix + funcName
		for _, appendedFuncs := range c.templateFuncs {
			if _, exists := appendedFuncs[name]; exists {
				if lo.Contains(defaultPluginNames, p.Name) {
					Log("[DEBUG] template function %s already exists by default plugins. skip", name)
					continue
				}
				return fmt.Errorf("template function %s already exists. set func_prefix to %s plugin", name, p.Name)
			}
		}
		modified[name] = f
	}
	c.templateFuncs = append(c.templateFuncs, modified)
	return nil
}

func (p ConfigPlugin) AppendJsonnetNativeFuncs(c *Config, funcs []*jsonnet.NativeFunction) error {
	for _, f := range funcs {
		f.Name = p.FuncPrefix + f.Name
		for _, appendedFuncs := range c.jsonnetNativeFuncs {
			if appendedFuncs.Name == f.Name {
				if lo.Contains(defaultPluginNames, p.Name) {
					Log("[DEBUG] jsonnet native function %s already exists by default plugins. skip", f.Name)
					continue
				}
				return fmt.Errorf("jsonnet native function %s already exists. set func_prefix to %s plugin", f.Name, p.Name)
			}
		}
		c.jsonnetNativeFuncs = append(c.jsonnetNativeFuncs, f)
	}
	return nil
}

func setupPluginTFState(ctx context.Context, p ConfigPlugin, c *Config) error {
	var loc string
	if p.Config["path"] != nil {
		path, ok := p.Config["path"].(string)
		if !ok {
			return errors.New("tfstate plugin requires path for tfstate file as a string")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.dir, path)
		}
		loc = path
	} else if p.Config["url"] != nil {
		u, ok := p.Config["url"].(string)
		if !ok {
			return errors.New("tfstate plugin requires url for tfstate URL as a string")
		}
		loc = u
	} else {
		return errors.New("tfstate plugin requires path or url for tfstate location")
	}

	lookup, err := tfstate.ReadURL(ctx, loc)
	if err != nil {
		return err
	}
	if err := p.AppendFuncMap(c, lookup.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, lookup.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}

func setupPluginCFn(ctx context.Context, p ConfigPlugin, c *Config) error {
	cache := sync.Map{}
	lookup := cfn.New(c.awsv2Config, &cache)
	if err := p.AppendFuncMap(c, lookup.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, lookup.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}

func setupPluginSSM(ctx context.Context, p ConfigPlugin, c *Config) error {
	cache := sync.Map{}
	lookup := ssm.New(c.awsv2Config, &cache)
	if err := p.AppendFuncMap(c, lookup.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, lookup.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}

func setupPluginSecretsManager(ctx context.Context, p ConfigPlugin, c *Config) error {
	lookup := secretsmanager.NewApp(c.awsv2Config)
	if err := p.AppendFuncMap(c, lookup.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, lookup.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}
