package models

// Auth parses a JSON auth request from the client
type Auth struct {
	Token string `json:"token"`
}

// MemberResponse parses the data from backend /member/self
type MemberResponse struct {
	S bool                   `json:"s"`
	D map[string]interface{} `json:"d"`
	M string                 `json:"m"`
}
