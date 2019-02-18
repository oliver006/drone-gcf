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

	EntryPoint string `json:"entrypoint"`
	Memory     string `json:"memory"`
	Region     string `json:"region"`
	Retry      string `json:"retry"`
	Runtime    string `json:"runtime"`
	Source     string `json:"source"`
	Timeout    string `json:"timeout"`
}
type Functions []Function

type Config struct {
	Action  string `json:"action"`
	DryRun  bool   `json:"-"`
	Dir     string `json:"-"`
	Project string `json:"-"`
	Token   string `json:"token"`
	Runtime string `json:"runtime"`

	Functions Functions `json:"functions"`
}

const (
	// location of temp key file in the ephemeral drone container that runs drone-gcf
	TokenFileLocation = "/tmp/token.json"
)

var (
	// populated by "go build"
	BuildDate string
	BuildHash string
)

func isValidRuntime(r string) bool {
	return map[string]bool{
		"nodejs6":  true,
		"nodejs8":  true,
		"python37": true,
		"go111":    true,
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
		log.Printf("Invalid runtime: %s - continuing anyway", f.Runtime)
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
		Dir:     filepath.Join(os.Getenv("DRONE_WORKSPACE"), os.Getenv("PLUGIN_DIR")),
		Action:  os.Getenv("PLUGIN_ACTION"),
		DryRun:  os.Getenv("PLUGIN_DRY_RUN") == "true",
		Project: os.Getenv("PLUGIN_PROJECT"),
		Runtime: os.Getenv("PLUGIN_RUNTIME"),
		Token:   os.Getenv("PLUGIN_TOKEN"),
	}

	if cfg.Action == "" {
		return nil, fmt.Errorf("Missing action")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("Missing token")
	}

	defaultRuntime := os.Getenv("PLUGIN_RUNTIME")
	if defaultRuntime == "" {
		defaultRuntime = "go111"
	}

	switch cfg.Action {
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

func runConfig(cfg *Config) error {
	e := NewEnviron(cfg.Dir, os.Environ(), os.Stdout, os.Stderr)
	if err := e.Run(cfg.DryRun, "gcloud", "auth", "activate-service-account", "--key-file", TokenFileLocation); err != nil {
		return err
	}

	args := []string{
		"--quiet",
		"functions",
		cfg.Action,
		"--project", cfg.Project,
	}

	switch cfg.Action {
	case "call":
		return fmt.Errorf("action: %s not implemented yet", cfg.Action)
	case "deploy":
		for _, f := range cfg.Functions {
			if !isValidFunctionForDeploy(f) {
				continue
			}

			runArgs := append(args, f.Name, "--runtime", f.Runtime)

			switch f.Trigger {
			case "bucket":
				runArgs = append(runArgs, "--trigger-bucket", f.TriggerResource)
			case "http":
				runArgs = append(runArgs, "--trigger-http")
			case "topic":
				runArgs = append(runArgs, "--trigger-topic", f.TriggerResource)
			case "event":
				runArgs = append(runArgs, "--trigger-event", f.TriggerEvent, "--trigger-resource="+f.TriggerResource)
			}

			if f.Source != "" {
				runArgs = append(runArgs, "--source", f.Source)
			}
			if f.Memory != "" {
				runArgs = append(runArgs, "--memory", f.Memory)
			}
			if f.EntryPoint != "" {
				runArgs = append(runArgs, "--entry-point", f.EntryPoint)
			}
			if f.Region != "" {
				runArgs = append(runArgs, "--region", f.Region)
			}
			if f.Retry != "" {
				runArgs = append(runArgs, "--retry", f.Retry)
			}
			if f.Timeout != "" {
				runArgs = append(runArgs, "--timeout", f.Timeout)
			}

			err := e.Run(cfg.DryRun, "gcloud", runArgs...)
			if err != nil {
				return fmt.Errorf("error: %s\n", err)
			}
		}

	case "delete":
		for _, f := range cfg.Functions {
			runArgs := append(args, f.Name)
			if f.Region != "" {
				runArgs = append(runArgs, "--region", f.Region)
			}
			err := e.Run(cfg.DryRun, "gcloud", runArgs...)
			if err != nil {
				return fmt.Errorf("error: %s\n", err)
			}
		}

	case "list":
		err := e.Run(cfg.DryRun, "gcloud", args...)
		if err != nil {
			return fmt.Errorf("error: %s\n", err)
		}

	default:
		return fmt.Errorf("action: %s not implemented yet", cfg.Action)
	}

	return nil
}

type Environ struct {
	dir    string
	env    []string
	stdout io.Writer
	stderr io.Writer
}

func NewEnviron(dir string, env []string, stdout, stderr io.Writer) *Environ {
	return &Environ{
		dir:    dir,
		env:    env,
		stdout: stdout,
		stderr: stderr,
	}
}

func (e *Environ) Run(dryRun bool, name string, arg ...string) error {
	log.Printf("Running: %s %#v", name, arg)
	if dryRun {
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

	if err := ioutil.WriteFile(TokenFileLocation, []byte(cfg.Token), 0600); err != nil {
		log.Fatalf("Error writing token file: %s", err)
	}

	defer func() {
		os.Remove(TokenFileLocation)
	}()

	if err := runConfig(cfg); err != nil {
		log.Fatalf("runConfig() err: %s", err)
		return
	}
}
