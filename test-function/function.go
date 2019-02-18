package function

import (
	"fmt"
	"net/http"
)

func TestDeployment(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "drone-gcf plugin, hash: %s", BuildHash)
}
