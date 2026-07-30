package jwtutil_test

import (
	"testing"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignAndParse_RoundTrip(t *testing.T) {
	signer := jwtutil.NewSigner("test-secret", 60)
	userID := uuid.New()

	token, err := signer.Sign(userID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := signer.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestParse_ExpiredToken(t *testing.T) {
	signer := jwtutil.NewSigner("test-secret", -1) // expiry already in the past
	userID := uuid.New()

	token, err := signer.Sign(userID)
	require.NoError(t, err)

	_, err = signer.Parse(token)
	assert.Error(t, err)
}

func TestParse_WrongSecret(t *testing.T) {
	signer := jwtutil.NewSigner("secret-a", 60)
	other := jwtutil.NewSigner("secret-b", 60)

	token, err := signer.Sign(uuid.New())
	require.NoError(t, err)

	_, err = other.Parse(token)
	assert.Error(t, err)
}

func TestParse_MalformedToken(t *testing.T) {
	signer := jwtutil.NewSigner("test-secret", 60)

	cases := []string{
		"",
		"not-a-jwt",
		"a.b.c",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			_, err := signer.Parse(tc)
			assert.Error(t, err)
		})
	}
}

func TestParse_WrongSigningMethod(t *testing.T) {
	signer := jwtutil.NewSigner("test-secret", 60)

	// Craft a token with the "none" algorithm - Parse must reject it even
	// though it would otherwise pass token.Valid, since the keyfunc callback
	// explicitly checks for *jwt.SigningMethodHMAC.
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = signer.Parse(signed)
	assert.Error(t, err)
}

func TestParse_UserIDNilWhenInvalid(t *testing.T) {
	signer := jwtutil.NewSigner("test-secret", 60)

	got, err := signer.Parse("garbage")
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, got)
}
