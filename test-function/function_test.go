package function

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFunctionDeployment(t *testing.T) {
	BuildHash := os.Getenv("BUILD_HASH")
	if BuildHash == "" {
		t.Errorf("Didn't find build hash env var")
		return
	}

	WhatWeWant := []string{
		BuildHash,
		"oh noes",
		`oh " my`,
		`oh , well`,
		"env_var_123",
		"secret-api-key-123",
	}

	for count := 15; count > 0; count-- {
		client := http.Client{
			Timeout: 10 * time.Second,
		}
		urlString := os.Getenv("GCF_URL")
		_, err := url.Parse(urlString)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", urlString, err)
		}

		if resp, err := client.Get(urlString); err == nil {
			if body, err := ioutil.ReadAll(resp.Body); err == nil {
				greatSuccess := true
				for _, want := range WhatWeWant {
					want = fmt.Sprintf("[%s]", want)
					if idx := strings.Index(string(body), want); idx != -1 {
						log.Printf("Found %s - great success", want)
					} else {
						log.Printf("didn't find %s   - body: [%s]", want, string(body))
						greatSuccess = false
					}
				}

				if greatSuccess {
					return
				}
			} else {
				log.Printf("ioutil.ReadAll: %v", err)
			}
		} else {
			log.Printf("http.Get: %v", err)
		}

		log.Printf("Going to sleep for 7 seconds and then will try %d more times", count)
		time.Sleep(7 * time.Second)
	}

	t.Errorf("Didn't find what we were looking for: %#v", WhatWeWant)
}
