package external

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ractf/socketwrench/config"
	"github.com/ractf/socketwrench/models"
)

// GetUser queries the backend for a given token
func GetUser(token string) (uint32, bool) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", config.BackendAddr+"member/self/", nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("Authorization", "Token "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	response, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return 0, false
	}
	var mr models.MemberResponse
	err = json.Unmarshal([]byte(response), &mr)
	if err != nil {
		return 0, false
	}
	id, pres := mr.D["id"]
	if !pres {
		return 0, false
	}
	fmt.Println(id)
	return uint32(id.(float64)), true
}
