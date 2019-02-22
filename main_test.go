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
		"TransferFile,Func1234,ThirdFunc",
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
		Env               map[string]string
		expectedToBeOk    bool
		expectedProjectId string
	}{
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": pf},
			expectedProjectId: "my-project-id",
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
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_RUNTIME": "python37", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "[{\"TransferFile\":[{\"trigger\":\"http\",\"runtime\":\"lol123\",\"memory\":\"2048MB\"}]}]"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "deploy", "PLUGIN_RUNTIME": "python37", "TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "[{\"TransferFile\":[{\"trigger\":\"http\",\"runtime\":\"lol123\",\"memory\":\"2048MB\"}]}]"},
			expectedProjectId: "my-project-id",
		},
		{
			expectedToBeOk:    true,
			Env:               map[string]string{"PLUGIN_ACTION": "delete", "PLUGIN_TOKEN": validGCPKey, "PLUGIN_FUNCTIONS": "DeleteFunction1,DeleteFunction2"},
			expectedProjectId: "my-project-id",
		},

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
	}
}

func TestEnvironRun(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	e := NewEnviron("/tmp", []string{"ABC=123"}, stdout, stderr)

	if err := e.Run(false, "/bin/echo", "sup"); err == nil {
		if stdout.String() != "sup\n" {
			t.Errorf("got stdout : %s", stdout.String())
		}
		if stderr.String() != "" {
			t.Errorf("got stdout : %s", stderr.String())
		}
	} else {
		t.Errorf("got err: %s", err)
	}

	if err := e.Run(false, "/usr/bin/env"); err == nil {
		if strings.Index(stdout.String(), "ABC=123") == -1 {
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
