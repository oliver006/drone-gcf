package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Function struct {
	Name            string `json:"name"`
	Trigger         string `json:"trigger"`
	TriggerEvent    string `json:"trigger_event"`
	TriggerResource string `json:"trigger_resource"`

	AllowUnauthenticated bool   `json:"allow_unauthenticated"`
	EntryPoint           string `json:"entrypoint"`
	Memory               string `json:"memory"`
	Region               string `json:"region"`
	Retry                string `json:"retry"`
	Runtime              string `json:"runtime"`
	Source               string `json:"source"`
	Timeout              string `json:"timeout"`
	ServiceAccount       string `json:"serviceaccount"`

	EnvironmentDelimiter string              `json:"environment_delimiter"`
	Environment          []map[string]string `json:"environment"`

	// used for action==call
	Data string
}

type Functions []Function

type Config struct {
	Action     string
	DryRun     bool
	Verbose    bool
	Dir        string
	Project    string
	Token      string
	Runtime    string
	Verbosity  string
	EnvSecrets []string
	Functions  Functions
}

const (
	// location of temp key file within the ephemeral drone container that runs drone-gcf
	TmpTokenFileLocation   = "/tmp/token.json"
	defaultEnvVarDelimiter = ":|:"
)

var (
	// populated by "go build"
	BuildDate string
	BuildHash string
	BuildTag  string
)

func isValidRuntime(r string) bool {
	return map[string]bool{
		"nodejs6":  true,
		"nodejs8":  true,
		"nodejs10": true,
		"nodejs12": true,
		"python37": true,
		"python38": true,
		"go111":    true,
		"go113":    true,
		"java11":   true,
	}[r]
}

func isValidTriggerType(t string) bool {
	return map[string]bool{
		"http":   true,
		"bucket": true,
		"topic":  true,
		"event":  true,
	}[t]
}

func isValidFunctionForDeploy(f Function) bool {
	if !isValidRuntime(f.Runtime) {
		log.Printf("Missing or invalid runtime [%s] for function: %s", f.Runtime, f.Name)
		return false
	}

	if f.Trigger == "http" {
		return true
	}

	if (f.Trigger == "" && f.TriggerEvent == "" && f.TriggerResource == "") || !isValidTriggerType(f.Trigger) {
		log.Printf("Missing or invalid trigger for function %s", f.Name)
		return false
	}

	if f.Trigger != "http" && f.TriggerResource == "" {
		log.Printf("Missing or invalid trigger resource for function %s", f.Name)
		return false
	}

	if f.Trigger == "event" && f.TriggerEvent == "" {
		log.Printf("Missing trigger event for function %s", f.Name)
		return false
	}

	return true
}

func parseFunctions(e string, defaultRuntime string) []Function {
	res := Functions{}
	d := []map[string]Functions{}
	if err := json.Unmarshal([]byte(e), &d); err != nil {
		if s := strings.Split(e, ","); e != "" && len(s) > 0 && s[0] != "" {
			for _, n := range s {
				n = strings.TrimSpace(n)
				if n != "" {
					res = append(res, Function{Name: n})
				}
			}
		}
		return res
	}

	for _, v := range d {
		for k, fs := range v {
			for _, f := range fs {
				f.Name = strings.TrimSpace(k)
				if f.Runtime == "" {
					f.Runtime = defaultRuntime
				}
				if f.EnvironmentDelimiter == "" {
					f.EnvironmentDelimiter = defaultEnvVarDelimiter
				}
				res = append(res, f)
			}
		}
	}
	return res
}

func getProjectFromToken(token string) string {
	data := struct {
		ProjectID string `json:"project_id"`
	}{}
	err := json.Unmarshal([]byte(token), &data)
	if err != nil {
		return ""
	}
	return data.ProjectID
}

func parseConfig() (*Config, error) {
	cfg := Config{
		Dir:       filepath.Join(os.Getenv("DRONE_WORKSPACE"), os.Getenv("PLUGIN_DIR")),
		Action:    os.Getenv("PLUGIN_ACTION"),
		DryRun:    os.Getenv("PLUGIN_DRY_RUN") == "true",
		Project:   os.Getenv("PLUGIN_PROJECT"),
		Runtime:   os.Getenv("PLUGIN_RUNTIME"),
		Token:     os.Getenv("PLUGIN_TOKEN"),
		Verbosity: os.Getenv("PLUGIN_VERBOSITY"),
	}

	if cfg.Action == "" {
		return nil, fmt.Errorf("Missing action")
	}
	if cfg.Verbosity == "" {
		cfg.Verbosity = "warning"
	}

	PluginEnvSecretPrefix := "PLUGIN_ENV_SECRET_"
	for _, e := range os.Environ() {
		if s := strings.SplitN(e, "=", 2); len(s) > 0 && strings.HasPrefix(s[0], PluginEnvSecretPrefix) {
			k := strings.TrimPrefix(s[0], PluginEnvSecretPrefix)
			v := os.Getenv(s[0])
			cfg.EnvSecrets = append(cfg.EnvSecrets, fmt.Sprintf(`%s=%s`, k, v))
		}
	}

	if cfg.Token == "" {
		cfg.Token = os.Getenv("TOKEN")
		if cfg.Token == "" {
			return nil, fmt.Errorf("Missing token")
		}
	}

	if cfg.Runtime == "" {
		cfg.Runtime = "go111"
	}

	switch cfg.Action {
	case "call":
		for _, f := range parseFunctions(os.Getenv("PLUGIN_FUNCTIONS"), cfg.Runtime) {
			cfg.Functions = append(cfg.Functions, f)
		}
	case "deploy":
		for _, f := range parseFunctions(os.Getenv("PLUGIN_FUNCTIONS"), cfg.Runtime) {
			if isValidFunctionForDeploy(f) {
				cfg.Functions = append(cfg.Functions, f)
			}
		}
	case "delete":
		cfg.Functions = parseFunctions(os.Getenv("PLUGIN_FUNCTIONS"), cfg.Runtime)
	}

	if len(cfg.Functions) == 0 && cfg.Action != "list" {
		return nil, fmt.Errorf("Didn't find any functions")
	}

	if cfg.Project == "" {
		cfg.Project = getProjectFromToken(cfg.Token)
		if cfg.Project == "" {
			return nil, fmt.Errorf("project id not found in token or param")
		}
	}

	log.Printf("Using project ID: %s", cfg.Project)

	return &cfg, nil
}

type Plan struct {
	Steps [][]string
}

func CreateExecutionPlan(cfg *Config) (Plan, error) {
	res := Plan{Steps: [][]string{}}

	baseArgs := []string{
		"--quiet",
		"functions",
		cfg.Action,
		"--project", cfg.Project,
		"--verbosity", cfg.Verbosity,
	}

	switch cfg.Action {
	case "call":
		for _, f := range cfg.Functions {
			args := append(baseArgs, f.Name)
			if f.Region != "" {
				args = append(args, "--region", f.Region)
			}
			if f.Data != "" {
				args = append(args, "--data", f.Data)
			}
			res.Steps = append(res.Steps, args)
		}

	case "deploy":
		for _, f := range cfg.Functions {
			if !isValidFunctionForDeploy(f) {
				return res, fmt.Errorf("invalid config for function: %s", f.Name)
			}

			args := append(baseArgs, f.Name, "--runtime", f.Runtime)

			switch f.Trigger {
			case "bucket":
				args = append(args, "--trigger-bucket", f.TriggerResource)
			case "http":
				args = append(args, "--trigger-http")
			case "topic":
				args = append(args, "--trigger-topic", f.TriggerResource)
			case "event":
				args = append(args, "--trigger-event", f.TriggerEvent, "--trigger-resource="+f.TriggerResource)
			}

			if f.AllowUnauthenticated {
				args = append(args, "--allow-unauthenticated")
			}
			if f.Source != "" {
				args = append(args, "--source", f.Source)
			}
			if f.Memory != "" {
				args = append(args, "--memory", f.Memory)
			}
			if f.EntryPoint != "" {
				args = append(args, "--entry-point", f.EntryPoint)
			}
			if f.Region != "" {
				args = append(args, "--region", f.Region)
			}
			if f.Retry != "" {
				args = append(args, "--retry", f.Retry)
			}
			if f.Timeout != "" {
				args = append(args, "--timeout", f.Timeout)
			}
			if f.ServiceAccount != "" {
				args = append(args, "--service-account", f.ServiceAccount)
			}
			if len(cfg.EnvSecrets) > 0 || len(f.Environment) > 0 {
				e := make([]string, len(cfg.EnvSecrets))
				copy(e, cfg.EnvSecrets)

				if len(f.Environment) > 0 {
					for k, v := range f.Environment[0] {
						e = append(e, fmt.Sprintf(`%s=%s`, k, v))
					}
				}

				envStr := "^" + f.EnvironmentDelimiter + "^" + strings.Join(e, f.EnvironmentDelimiter)
				args = append(args, "--set-env-vars", envStr)
			}

			res.Steps = append(res.Steps, args)
		}

	case "delete":
		for _, f := range cfg.Functions {
			args := append(baseArgs, f.Name)
			if f.Region != "" {
				args = append(args, "--region", f.Region)
			}
			res.Steps = append(res.Steps, args)
		}

	case "list":
		res.Steps = append(res.Steps, baseArgs)

	default:
		return res, fmt.Errorf("action: %s not implemented yet", cfg.Action)
	}

	return res, nil
}

func ExecutePlan(e *Env, plan Plan) error {
	for _, args := range plan.Steps {
		if err := e.Run("gcloud", args...); err != nil {
			return fmt.Errorf("error: %s\n", err)
		}
	}

	return nil
}

func runConfig(cfg *Config) error {
	plan, err := CreateExecutionPlan(cfg)
	if err != nil {
		return err
	}

	e := NewEnv(cfg.Dir, os.Environ(), os.Stdout, os.Stderr, cfg.DryRun, cfg.Verbose)

	if err := e.Run("gcloud", "version"); err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	if err := e.Run("gcloud", "auth", "activate-service-account", "--key-file", TmpTokenFileLocation); err != nil {
		return err
	}

	return ExecutePlan(e, plan)
}

type Env struct {
	dir     string
	env     []string
	stdout  io.Writer
	stderr  io.Writer
	dryRun  bool
	verbose bool
}

func NewEnv(dir string, env []string, stdout, stderr io.Writer, dryRun bool, verbose bool) *Env {
	return &Env{
		dir:     dir,
		env:     env,
		stdout:  stdout,
		stderr:  stderr,
		dryRun:  dryRun,
		verbose: verbose,
	}
}

func (e *Env) Run(name string, arg ...string) error {
	if e.verbose {
		log.Printf("Running: %s %#v", name, arg)
	}
	if e.dryRun {
		return nil
	}
	cmd := exec.Command(name, arg...)
	cmd.Dir = e.dir
	cmd.Env = e.env
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr
	return cmd.Run()
}

func main() {
	if BuildTag == "" {
		BuildTag = "[not-tagged]"
	}
	log.Printf("Drone-GCF Plugin  version: %s   hash: %s   date: %s", BuildTag, BuildHash, BuildDate)

	showVersion := flag.Bool("v", false, "show version and exit")
	flag.Parse()

	if *showVersion {
		os.Exit(0)
		return
	}

	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("parseConfig() err: %s", err)
		return
	}

	if err := ioutil.WriteFile(TmpTokenFileLocation, []byte(cfg.Token), 0600); err != nil {
		log.Fatalf("Error writing token file: %s", err)
	}

	defer func() {
		os.Remove(TmpTokenFileLocation)
	}()

	if err := runConfig(cfg); err != nil {
		log.Fatalf("runConfig() err: %s", err)
		return
	}
}
