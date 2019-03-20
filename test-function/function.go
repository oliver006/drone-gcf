package function

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	stdLogger = log.New(os.Stdout, "", 0)
)

func TestDeployment(w http.ResponseWriter, r *http.Request) {
	BuildHash := os.Getenv("BUILD_HASH")
	stdLogger.Printf("BuildHash: %s", BuildHash)

	RegularEnvVar := os.Getenv("REGULAR_ENV_VAR")
	stdLogger.Printf("RegularEnvVar: %s", RegularEnvVar)

	ApiKey := os.Getenv("API_KEY")
	stdLogger.Printf("ApiKey: %s", ApiKey)

	fmt.Fprintf(w,
		"drone-gcf plugin, hash: [%s]     api key: [%s]         regular env var: [%s]      other env vars: [%s] [%s] [%s]",
		BuildHash,
		ApiKey,
		RegularEnvVar,
		os.Getenv("ENV_VAR_WITH_SPACE"),
		os.Getenv("ENV_VAR_WITH_QUOTES"),
		os.Getenv("ENV_VAR_WITH_COMMA"),
	)
}
