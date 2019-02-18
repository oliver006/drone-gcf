package function

import (
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
	for count := 10; count > 0; count-- {
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
				if idx := strings.Index(string(body), BuildHash); idx != -1 {
					log.Printf("Found hash: %s - great success", BuildHash)
					return
				} else {
					log.Printf("didn't find hash %s   - body: %s", BuildHash, string(body))
				}
			} else {
				log.Printf("ioutil.ReadAll: %v", err)
			}
		} else {
			log.Printf("http.Get: %v", err)
		}
		log.Printf("Sleep for 3 seconds and try again")
		time.Sleep(3 * time.Second)
	}
}
