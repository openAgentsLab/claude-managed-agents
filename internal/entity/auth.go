package entity

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the response body for a successful POST /auth/login.
type LoginResponse struct {
	Token string `json:"token"`
}
