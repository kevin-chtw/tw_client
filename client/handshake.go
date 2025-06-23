package client

// HandshakeClientData represents information about the client sent on the handshake.
type HandshakeClientData struct {
	Platform    string `json:"platform"`
	LibVersion  string `json:"libVersion"`
	BuildNumber string `json:"clientBuildNumber"`
	Version     string `json:"clientVersion"`
}

type SessionHandshakeData struct {
	Sys  HandshakeClientData    `json:"sys"`
	User map[string]interface{} `json:"user,omitempty"`
}

var sessionHandshake *SessionHandshakeData

func init() {
	sessionHandshake = &SessionHandshakeData{
		Sys: HandshakeClientData{
			Platform:    "repl",
			LibVersion:  "0.3.5-release",
			BuildNumber: "20",
			Version:     "1.0.0",
		},
		User: map[string]interface{}{
			"client": "repl",
		},
	}
}
