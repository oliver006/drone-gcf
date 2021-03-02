package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

var (
	validGCPKey = `
{
  "type": "service_account",
  "project_id": "my-project-id",
  "private_key_id": "",
  "private_key": "",
  "client_email": "my-project@appspot.gserviceaccount.com",
  "client_id": "123",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-project%40appspot.gserviceaccount.com"
}
`

	invalidGCPKey = `
{
  "type": "service_account",
  234: "invalid Json    ,

}
`
)

func TestParseFunctionsForDeploy(t *testing.T) {
	for _, tst := range []string{
		"[{\"TransferFile\":[{\"trigger\":\"http\"}]}]",
		"[{\"TransferFilePublic\":[{\"trigger\":\"http\",\"allow_unauthenticated\":true}]}]",
		"[{\"TransferFilePrivate\":[{\"trigger\":\"http\",\"allow_unauthenticated\":false}]}]",
		"[{\"TransferFile\":[{\"trigger\":\"http\",\"memory\":\"2048MB\"}]}]",
		"[{\"HeyNow123\":[{\"trigger\":\"bucket\",\"trigger_resource\":\"gs://my-bucket\",\"memory\":\"512MB\"}]}]",
		"[{\"Func654\":[{\"trigger\":\"topic\",\"trigger_resource\":\"topic/my-bucket\",\"memory\":\"512MB\"}]}]",
		"[{\"FuncNew\":[{\"trigger\":\"event\",\"trigger_event\":\"providers/cloud.storage/eventTypes/object.change\",\"trigger_resource\":\"gs://bucket321\"}]}]",
	} {
		functions := parseFunctions(tst, "go111")
		if len(functions) == 0 {
			t.Errorf("not enough functions")
			return
		}
		for _, f := range functions {
			if !isValidFunctionForDeploy(f) {
				t.Errorf("found an invalid function: %s", f.Name)
			}
		}
	}
}

func TestParseFunctionsForDelete(t *testing.T) {
	for _, tst := range []string{
		"TransferFile,ProcessEvents4,ThirdFunc",
	} {
		functions := parseFunctions(tst, "go111")
		if len(functions) == 0 {
			t.Errorf("not enough functions")
			return
		}
	}

}

func TestParseFunctionsforDeploy(t *testing.T) {
	for _, tst := range []string{
		"[{\"TransferFile\":[{\"t\":\"http\"}]}]",
		"[{\"HeyNow123\":[{\"trigger\":\"bucket\",\"trigger_resource\":\"\",\"memory\":\"512MB\"}]}]",
		"[{\"FuncNew\":[{\"trigger\":\"event\",\"trigger_event\":\"\",\"trigger_resource\":\"gs://bucket321\"}]}]",
	} {
		functions := parseFunctions(tst, "go111")
		for _, f := range functions {
			if isValidFunctionForDeploy(f) {
				t.Errorf("Should have rejected function: %s", f.Name)
			}
		}
	}

}

func TestParseConfig(t *testing.T) {
	pf := "[{\"TransferFile\":[{\"trigger\":\"http\",\"runtime\":\"go111\",\"memory\":\"2048MB\"}]}]"
	for _, tst := range []struct {
		Env                map[string]string
		expectedToBeOk     bool
		expectedProjectId  string
		expectedEnvSecrets []string
	}{
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf, "PLUGIN_ENV_SECRET_API_KEY": "api-key-123"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:     true,
			Env:                map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf, "PLUGIN_ENV_SECRET_API_KEY": "secret-api-key"},
			expectedProjectId:  "my-project-id",
			expectedEnvSecrets: []string{"API_KEY=secret-api-key"},
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_PROJECT": "project-2", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf},
			expectedProjectId: "project-2",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_RUNTIME": "python37", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "[{\"TransferFile\":[{\"trigger\":\"http\",\"memory\":\"2048MB\"}]}]"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_RUNTIME": "python37", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "[{\"TransferFile\":[{\"trigger\":\"http\",\"runtime\":\"python37\",\"memory\":\"2048MB\"}]}]"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_RUNTIME": "python37", "TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "[{\"TransferFile\":[{\"trigger\":\"http\",\"runtime\":\"python37\",\"memory\":\"2048MB\"}]}]"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "delete", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "DeleteFunction1,DeleteFunction2"},
			expectedProjectId: "my-project-id",
		},

		// [{"TransferFile":[{"environment":[{"ENV_KEY_01":"env_key_01"},{"ENV_KEY_02":"env_key_02"},{"ENV_KEY_03":"env_key_03"}],"memory":"2048MB","runtime":"go111","trigger":"http"}]}]

		/*
			broken configs
		*/
		{
			expectedToBeOk:    false,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "MISSING---TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    false,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_TOKEN": invalidGCPKey, "PLUGIN_FUNCTIONS": pf},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    false,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_RUNTIME": "python37", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "[{\"TransferFile\":[{\"trigger\":\"http\",\"runtime\":\"lol123\",\"memory\":\"2048MB\"}]}]"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    false,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_RUNTIME": "python37", "TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "[{\"TransferFile\":[{\"trigger\":\"http\",\"runtime\":\"lol123\",\"memory\":\"2048MB\"}]}]"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk: false,
			Env:            map[string]string{"": ""},
		},
		{
			expectedToBeOk: false,
			Env:            map[string]string{"PLUGIN_ACTION": "deploy", "TOOOOOOOOOOKEN": invalidGCPKey},
		},
	} {
		os.Clearenv()

		for k, v := range tst.Env {
			os.Setenv(k, v)
		}

		cfg, err := parseConfig()
		if err != nil && tst.expectedToBeOk == true {
			t.Errorf("parseConfig(  %#v  ) failed, err: %s", tst, err)
			return
		}
		if err == nil && tst.expectedToBeOk == false {
			t.Errorf("parseConfig(  %#v  ) should have failed", tst)
			return
		}
		if !tst.expectedToBeOk {
			continue
		}

		if cfg.Project != tst.expectedProjectId {
			t.Errorf("expected projectID: %s   got: %s", tst.expectedProjectId, cfg.Project)
		}

		for _, e := range tst.expectedEnvSecrets {
			found := false
			for _, s := range cfg.EnvSecrets {
				if s == e {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing env secret: %s, got: %#v", e, cfg.EnvSecrets)
			}
		}
	}
}

func TestExecutePlan(t *testing.T) {
	pId := "my-project-123"
	for _, tst := range []struct {
		cfg            Config
		expectedToBeOk bool
		expectedPlan   [][]string
	}{
		{
			cfg: Config{
				Action: "deploy",
				Functions: Functions{
					{
						Name:                 "ProcessEvents",
						Runtime:              "go111",
						Trigger:              "http",
						Memory:               "512MB",
						Timeout:              "20s",
						AllowUnauthenticated: true,
					},
					{
						Name:            "ProcessPubSub",
						Runtime:         "python37",
						Trigger:         "topic",
						TriggerResource: "topic/emails/filtered",
						Memory:          "2048MB",
						Timeout:         "20s",
					},
					{
						Name:            "ProcessNews",
						Runtime:         "go111",
						Trigger:         "bucket",
						TriggerResource: "gs://bucket/files/cool",
						Source:          "src/",
						Region:          "us-east1",
						Retry:           "3",
					},
					{
						Name:            "ProcessMoreEvents",
						Runtime:         "go111",
						Trigger:         "event",
						TriggerResource: "my.trigger.resource",
						TriggerEvent:    "my.event",
						EntryPoint:      "FuncEntryPoint",
					},
					{
						Name:           "ProcessEventsWithDifferentSA",
						Runtime:        "nodejs10",
						Trigger:        "http",
						Memory:         "512MB",
						Timeout:        "20s",
						ServiceAccount: "account@project.iam.gserviceaccount.com",
					},
				},
			},
			expectedToBeOk: true,
			expectedPlan: [][]string{
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessEvents", "--runtime", "go111", "--trigger-http", "--allow-unauthenticated", "--memory", "512MB", "--timeout", "20s"},
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessPubSub", "--runtime", "python37", "--trigger-topic", "topic/emails/filtered", "--memory", "2048MB", "--timeout", "20s"},
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessNews", "--runtime", "go111", "--trigger-bucket", "gs://bucket/files/cool", "--source", "src/", "--region", "us-east1", "--retry", "3"},
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessMoreEvents", "--runtime", "go111", "--trigger-event", "my.event", "--trigger-resource=my.trigger.resource", "--entry-point", "FuncEntryPoint"},
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessEventsWithDifferentSA", "--runtime", "nodejs10", "--trigger-http", "--memory", "512MB", "--timeout", "20s", "--service-account", "account@project.iam.gserviceaccount.com"},
			},
		},

		{
			cfg: Config{
				Action:     "deploy",
				EnvSecrets: []string{"ENV_SECRET_123=WUT"},
				Functions: Functions{
					{
						Name:                 "ProcessEvents",
						Runtime:              "go111",
						Trigger:              "http",
						Memory:               "512MB",
						EnvironmentDelimiter: ":|:",
						Environment:          []map[string]string{{"K": "V"}},
					},
				},
			},
			expectedToBeOk: true,
			expectedPlan: [][]string{
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessEvents", "--runtime", "go111", "--trigger-http", "--memory", "512MB", "--set-env-vars", "^:|:^ENV_SECRET_123=WUT:|:K=V"},
			},
		},

		{
			cfg: Config{
				Action:     "deploy",
				EnvSecrets: []string{"ENV_SECRET_123=WUT"},
				Functions: Functions{
					{
						Name:                 "ProcessEvents",
						Runtime:              "go111",
						Trigger:              "http",
						Memory:               "512MB",
						EnvironmentDelimiter: "~%~",
						Environment:          []map[string]string{{"K": "V"}},
					},
				},
			},
			expectedToBeOk: true,
			expectedPlan: [][]string{
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessEvents", "--runtime", "go111", "--trigger-http", "--memory", "512MB", "--set-env-vars", "^~%~^ENV_SECRET_123=WUT~%~K=V"},
			},
		},

		{
			cfg: Config{
				Action: "deploy",
				Functions: Functions{
					{
						Name:                 "ProcessEvents",
						Runtime:              "go111",
						Trigger:              "http",
						Memory:               "512MB",
						AllowUnauthenticated: false,
						EnvironmentDelimiter: ":|:",
						Environment:          []map[string]string{{"K": "V"}},
					},
				},
			},
			expectedToBeOk: true,
			expectedPlan: [][]string{
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessEvents", "--runtime", "go111", "--trigger-http", "--memory", "512MB", "--set-env-vars", "^:|:^K=V"},
			},
		},

		{
			cfg: Config{
				Action:     "deploy",
				EnvSecrets: []string{"ENV_SECRET_123=WUT"},
				Functions: Functions{
					{
						Name:                 "ProcessEvents",
						Runtime:              "go111",
						Trigger:              "http",
						Memory:               "512MB",
						EnvironmentDelimiter: ":|:",
					},
				},
			},
			expectedToBeOk: true,
			expectedPlan: [][]string{
				{"--quiet", "functions", "deploy", "--project", pId, "--verbosity", "info", "ProcessEvents", "--runtime", "go111", "--trigger-http", "--memory", "512MB", "--set-env-vars", "^:|:^ENV_SECRET_123=WUT"},
			},
		},

		{
			cfg: Config{
				Action: "delete",
				Functions: Functions{
					{
						Name: "ProcessEvents",
					},
					{
						Name:   "Func567",
						Region: "us-east1",
					},
				},
			},
			expectedToBeOk: true,
			expectedPlan:   [][]string{{"--quiet", "functions", "delete", "--project", "my-project-123", "--verbosity", "info", "ProcessEvents"}, {"--quiet", "functions", "delete", "--project", "my-project-123", "--verbosity", "info", "Func567", "--region", "us-east1"}},
		},

		{
			cfg: Config{
				Action: "list",
			},
			expectedToBeOk: true,
			expectedPlan:   [][]string{{"--quiet", "functions", "list", "--project", pId, "--verbosity", "info"}},
		},

		{
			cfg: Config{
				Action: "deploy",
				Functions: Functions{
					{
						Name:    "ProcessNews",
						Runtime: "go111",
						Trigger: "bucket",
					},
				},
			},
			expectedToBeOk: false,
		},

		{
			cfg: Config{
				Action: "call",
			},
			expectedToBeOk: false,
		},
		{
			cfg: Config{
				Action: "invalid",
			},
			expectedToBeOk: false,
		},
		{
			cfg: Config{
				Action: "call",
				Functions: Functions{
					{
						Name: "UpdateDatabase",
						Data: `{"key": "value"}`,
					},
				},
			},
			expectedToBeOk: true,
			expectedPlan:   [][]string{{"--quiet", "functions", "call", "--project", pId, "--verbosity", "info", "UpdateDatabase", "--data", `{"key": "value"}`}},
		},
	} {
		tst.cfg.Project = pId
		tst.cfg.Verbosity = "info"
		plan, err := CreateExecutionPlan(&tst.cfg)
		if err != nil && tst.expectedToBeOk == true {
			t.Fatalf("CreateExecutionPlan(  %#v  ) failed, err: %s", tst, err)
		}

		if len(plan.Steps) != len(tst.expectedPlan) {
			t.Fatalf("not matching,\n\n   got: %#v              \nwanted: %#v", plan.Steps, tst.expectedPlan)
		}

		for i := range plan.Steps {
			if len(plan.Steps[i]) != len(tst.expectedPlan[i]) {
				t.Fatalf("not matching number of args,\n\n   got: %#v              \nwanted: %#v", plan.Steps[i], tst.expectedPlan[i])
			}

			for j := range plan.Steps[i] {
				if plan.Steps[i][j] != tst.expectedPlan[i][j] {
					t.Fatalf("not matching args, got [%s]   expected: [%s]", plan.Steps[i][j], tst.expectedPlan[i][j])
				}
			}
		}

	}
}

func TestEnvironRun(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	e := NewEnv("/tmp", []string{"ABC=123"}, stdout, stderr, false, true)

	if err := e.Run("/bin/echo", "sup"); err == nil {
		if stdout.String() != "sup\n" {
			t.Errorf("got stdout : %s", stdout.String())
		}
		if stderr.String() != "" {
			t.Errorf("got stdout : %s", stderr.String())
		}
	} else {
		t.Errorf("got err: %s", err)
	}

	if err := e.Run("/usr/bin/env"); err == nil {
		if !strings.Contains(stdout.String(), "ABC=123") {
			t.Errorf("didn't find ABC in Env, got: %s", stdout.String())
		}
	} else {
		t.Errorf("got err: %s", err)
	}
}

func TestGetProjectFromToken(t *testing.T) {
	if id := getProjectFromToken(validGCPKey); id != "my-project-id" {
		t.Errorf("Wrong project id, got: %s", id)
	}

	if id := getProjectFromToken(invalidGCPKey); id != "" {
		t.Errorf("Expected empty id, got: %s", id)
	}
}
