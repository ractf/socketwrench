package external

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/ractf/socketwrench/config"
	"github.com/ractf/socketwrench/models"
)

// GetUser queries the backend for a given token
func GetUser(token string) (uint32, bool) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", config.BackendAddr+"/member/self", nil)
	if err != nil {
		log.Print("could not make request:", err)
		return 0, false
	}
	req.Header.Set("Authorization", "Token "+token)
	res, err := client.Do(req)
	if err != nil {
		log.Print("could not fetch backend:", err)
		return 0, false
	}
	response, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Print("could not read body:", err)
		return 0, false
	}
	var mr models.MemberResponse
	err = json.Unmarshal([]byte(response), &mr)
	if err != nil {
		log.Print("could not unmarshal json:", err)
		return 0, false
	}
	id, pres := mr.D["id"]
	if !pres {
		log.Print("auth failed with bad token")
		return 0, false
	}
	return uint32(id.(float64)), true
}
