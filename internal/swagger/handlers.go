package swagger

import (
	"encoding/json"
	"net/http"

	genapi "github.com/USSTM/cv-backend/generated/api"
)

// OpenAPI spec as JSON (LLMs???????????)
func ServeSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	spec, err := genapi.GetSwagger()
	if err != nil {
		http.Error(w, "Failed to load OpenAPI spec", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS off for docs
	json.NewEncoder(w).Encode(spec)
}
