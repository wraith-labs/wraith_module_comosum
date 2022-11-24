package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cristalhq/jwt/v4"
	"github.com/google/uuid"
)

type authStatus int

const (
	// The order here is important and means that, if other
	// access levels are added in future, existing sessions
	// will stay the same or be demoted but not promoted.
	// Always put the access levels in order of lowest to
	// highest privilege!
	AUTH_STATUS_N authStatus = iota // None
	AUTH_STATUS_V                   // View
	AUTH_STATUS_A                   // Admin
)

const (
	SESSION_TOKEN_VALIDITY = 2 * time.Hour
)

var authKey []byte
var authSigner jwt.Signer
var authVerifier jwt.Verifier

func init() {
	//
	// Init variables needed for auth.
	//

	authKey = make([]byte, 512)
	io.ReadFull(rand.Reader, authKey)

	var err error
	authSigner, err = jwt.NewSignerHS(jwt.HS512, authKey)
	if err != nil {
		panic(fmt.Sprintf("failed to create JWT signer: %e", err))
	}
	authVerifier, err = jwt.NewVerifierHS(jwt.HS512, authKey)
	if err != nil {
		panic(fmt.Sprintf("failed to create JWT verifier: %e", err))
	}
}

func CreateSessionToken(status authStatus) ([]byte, time.Time) {
	expiry := time.Now().UTC().Add(SESSION_TOKEN_VALIDITY)

	claims := &jwt.RegisteredClaims{
		ID:        uuid.New().String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(expiry),
		Subject:   fmt.Sprint(status),
	}

	builder := jwt.NewBuilder(authSigner)

	token, err := builder.Build(claims)
	if err != nil {
		panic(fmt.Sprintf("failed to generate session token: %e", err))
	}

	return token.Bytes(), expiry
}

func VerifySessionToken(tokenBytes []byte) authStatus {
	token, err := jwt.Parse(tokenBytes, authVerifier)
	if err != nil {
		return AUTH_STATUS_N
	}

	var claims jwt.RegisteredClaims
	err = json.Unmarshal(token.Claims(), &claims)
	if err != nil {
		return AUTH_STATUS_N
	}

	status, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return AUTH_STATUS_N
	}

	return authStatus(status)
}

func ExtractSessionToken(r *http.Request) []byte {
	authHeader, exists := r.Header["Authorization"]
	if !exists {
		return []byte{}
	}

	for _, value := range authHeader {
		parts := strings.Split(value, " ")

		if len(parts) != 2 {
			continue
		}

		return []byte(parts[1])
	}

	return []byte{}
}

func AuthStatus(r *http.Request) authStatus {
	return VerifySessionToken(
		ExtractSessionToken(r),
	)
}

func StatusInGroup(status authStatus, group ...authStatus) bool {
	for _, gstatus := range group {
		if status == gstatus {
			return true
		}
	}

	return false
}

func TradeTokens(c Config, intoken []byte) ([]byte, time.Time, authStatus, bool) {
	// If the admin and view tokens are the same, the issued session will
	// have view privileges for security.
	switch string(intoken) {
	case "":
		// Special case to effectively disable access levels without a token.
		return []byte{}, time.Time{}, AUTH_STATUS_N, false
	case c.panelViewToken:
		token, expiry := CreateSessionToken(AUTH_STATUS_V)
		return token, expiry, AUTH_STATUS_V, true
	case c.panelAdminToken:
		token, expiry := CreateSessionToken(AUTH_STATUS_A)
		return token, expiry, AUTH_STATUS_A, true
	default:
		return []byte{}, time.Time{}, AUTH_STATUS_N, false
	}
}
