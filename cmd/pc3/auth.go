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
	AUTH_STATUS_N authStatus = iota
	AUTH_STATUS_A
	AUTH_STATUS_V
)

const (
	SESSION_TOKEN_VALIDITY = time.Hour
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

func CreateSessionToken(status authStatus) []byte {
	claims := &jwt.RegisteredClaims{
		ID:       uuid.New().String(),
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(
			time.Now().Add(SESSION_TOKEN_VALIDITY),
		),
		Subject: fmt.Sprint(status),
	}

	builder := jwt.NewBuilder(authSigner)

	token, err := builder.Build(claims)
	if err != nil {
		panic(fmt.Sprintf("failed to generate session token: %e", err))
	}

	return token.Bytes()
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

func AuthStatus(r *http.Request) authStatus {
	authHeader, exists := r.Header["Authorization"]
	if !exists {
		return AUTH_STATUS_N
	}

	for _, value := range authHeader {
		parts := strings.Split(value, " ")

		if len(parts) != 2 {
			continue
		}

		if status := VerifySessionToken([]byte(parts[1])); status != AUTH_STATUS_N {
			return status
		}
	}

	return AUTH_STATUS_N
}

func StatusInGroup(status authStatus, group []authStatus) bool {
	for _, gstatus := range group {
		if status == gstatus {
			return true
		}
	}

	return false
}

func TradeTokens(c Config, intoken []byte) ([]byte, bool) {

	switch string(intoken) {
	case "":
		// Special case to effectively disable accounts without a token.
		return []byte{}, false
	case c.panelAdminToken:
		return CreateSessionToken(AUTH_STATUS_A), true
	case c.panelViewToken:
		return CreateSessionToken(AUTH_STATUS_V), true
	default:
		return []byte{}, false
	}
}
