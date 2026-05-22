package orchestration

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"forge/internal/entity"
)

const identityKey = "identity"

// forgeClaims extends the standard JWT claims with forge-specific fields.
type forgeClaims struct {
	jwtv5.RegisteredClaims
	TenantID string `json:"tid"`
	Role     string `json:"role"`
}

// Identity is the decoded result of a validated JWT request.
type Identity struct {
	UserID   string // tenantID + "/" + username (scoped internal ID)
	TenantID string
	Role     string
}

// authHandler owns JWT configuration (secret + TTL) and exposes sign/verify
// helpers used by handleLogin and authMiddleware. Keeping auth state separate
// from HTTPOrchestrator improves cohesion and makes the auth logic independently
// testable.
type authHandler struct {
	secret []byte
	ttl    time.Duration
}

func (a authHandler) sign(sub, tenantID, role string) (string, error) {
	return generateToken(a.secret, sub, tenantID, role, a.ttl)
}

func (a authHandler) verify(tokenStr string) (*forgeClaims, error) {
	var claims forgeClaims
	_, err := jwtv5.ParseWithClaims(tokenStr, &claims, func(t *jwtv5.Token) (any, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	}, jwtv5.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

func (o *HTTPOrchestrator) handleLogin(c *gin.Context) {
	var req entity.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	foundTenant, foundUser, storeErr := o.tenantStore.Users().FindByUsername(c.Request.Context(), req.Username)

	// Always run bcrypt to prevent timing-based username enumeration.
	hash := "$2a$10$aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	found := storeErr == nil && foundUser != nil
	if found {
		hash = foundUser.PasswordHash
	}
	bcryptErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password))

	if storeErr != nil {
		c.String(http.StatusInternalServerError, "internal server error")
		return
	}
	if bcryptErr != nil || !found {
		c.String(http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Internal user ID embeds the tenant to guarantee global uniqueness across
	// all storage layers (session keys, memory, sandbox pool, workspace paths).
	internalUserID := foundTenant.ID + "/" + foundUser.Username
	role := foundUser.Role
	if role == "" {
		role = entity.RoleMember
	}
	token, err := o.auth.sign(internalUserID, foundTenant.ID, role)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to generate token")
		return
	}

	c.JSON(http.StatusOK, entity.LoginResponse{Token: token})
}

func (o *HTTPOrchestrator) handleLogout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// authMiddleware validates the Bearer JWT and sets Identity in the Gin context.
func (o *HTTPOrchestrator) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, entity.AuthBearerPrefix) {
			c.String(http.StatusUnauthorized, "missing or malformed Authorization header")
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(raw, entity.AuthBearerPrefix)

		claims, err := o.auth.verify(tokenStr)
		if err != nil {
			if errors.Is(err, jwtv5.ErrTokenExpired) {
				c.String(http.StatusUnauthorized, "token expired")
			} else {
				c.String(http.StatusUnauthorized, "invalid token: %s", err.Error())
			}
			c.Abort()
			return
		}

		sub, err := claims.GetSubject()
		if err != nil || sub == "" {
			c.String(http.StatusUnauthorized, "token missing sub claim")
			c.Abort()
			return
		}

		c.Set(identityKey, Identity{
			UserID:   sub,
			TenantID: claims.TenantID,
			Role:     claims.Role,
		})
		c.Next()
	}
}

// adminMiddleware aborts with 403 when the caller is not an admin.
// Must be used after authMiddleware so identityKey is already set.
func (o *HTTPOrchestrator) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.MustGet(identityKey).(Identity)
		if id.Role != entity.RoleAdmin {
			c.String(http.StatusForbidden, "admin role required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// generateToken creates a signed HS256 JWT with the forge-specific claims.
func generateToken(secret []byte, sub, tenantID, role string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := forgeClaims{
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   sub,
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(now.Add(expiry)),
		},
		TenantID: tenantID,
		Role:     role,
	}
	return jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString(secret)
}
