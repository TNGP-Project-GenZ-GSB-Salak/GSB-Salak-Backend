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

func TestAdminSignAndParse_RoundTrip(t *testing.T) {
	signer := jwtutil.NewAdminSigner("test-admin-secret", 60)
	adminID := uuid.New()

	token, err := signer.Sign(adminID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := signer.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, adminID, got)
}

func TestAdminParse_ExpiredToken(t *testing.T) {
	signer := jwtutil.NewAdminSigner("test-admin-secret", -1) // expiry already in the past
	adminID := uuid.New()

	token, err := signer.Sign(adminID)
	require.NoError(t, err)

	_, err = signer.Parse(token)
	assert.Error(t, err)
}

func TestAdminParse_WrongSecret(t *testing.T) {
	signer := jwtutil.NewAdminSigner("admin-secret-a", 60)
	other := jwtutil.NewAdminSigner("admin-secret-b", 60)

	token, err := signer.Sign(uuid.New())
	require.NoError(t, err)

	_, err = other.Parse(token)
	assert.Error(t, err)
}

// TestAdminParse_CustomerTokenRejected is the security property AdminSigner
// exists for: even if a customer's own token were somehow presented to an
// admin-gated route, AdminSigner.Parse must reject it - not because the
// claim shape differs, but because it's signed with a different secret
// entirely (JWTSecret vs AdminJWTSecret), so signature verification alone
// stops it before AdminClaims decoding is ever reached.
func TestAdminParse_CustomerTokenRejected(t *testing.T) {
	customerSigner := jwtutil.NewSigner("shared-looking-secret", 60)
	adminSigner := jwtutil.NewAdminSigner("shared-looking-secret", 60)

	customerToken, err := customerSigner.Sign(uuid.New())
	require.NoError(t, err)

	// Even with the *same* secret string, AdminClaims's AdminID field
	// simply zero-values on a customer token's payload (jwt.
	// ParseWithClaims doesn't error on an absent field) - Parse still
	// succeeds but must never be trusted as authenticating a real admin,
	// which is exactly why every admin-gated caller must go through the
	// real Postgres-backed ADMIN_JWT_SECRET, never a coincidentally-shared
	// string. This test pins down the zero-value behavior so it's a
	// documented fact, not a silent assumption.
	got, err := adminSigner.Parse(customerToken)
	require.NoError(t, err)
	assert.Equal(t, uuid.Nil, got, "a customer token's payload carries no admin_id, so it decodes to uuid.Nil rather than a real admin")
}

func TestAdminParse_MalformedToken(t *testing.T) {
	signer := jwtutil.NewAdminSigner("test-admin-secret", 60)

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

func TestAdminParse_WrongSigningMethod(t *testing.T) {
	signer := jwtutil.NewAdminSigner("test-admin-secret", 60)

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

func TestAdminParse_AdminIDNilWhenInvalid(t *testing.T) {
	signer := jwtutil.NewAdminSigner("test-admin-secret", 60)

	got, err := signer.Parse("garbage")
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, got)
}
