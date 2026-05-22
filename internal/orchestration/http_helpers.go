package orchestration

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (o *HTTPOrchestrator) handleHealth(c *gin.Context) {
	c.Status(http.StatusOK)
}

// scopedSessionID namespaces the client-visible session ID by scoped userID
// so the flat SessionStore achieves per-tenant-per-user isolation.
// userID here is already the scoped form: "{tenantID}/{username}".
func scopedSessionID(userID, sessionID string) string {
	return userID + ":" + sessionID
}

func writeSSEJSON(w io.Writer, payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func newSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
