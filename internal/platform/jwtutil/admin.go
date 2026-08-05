package jwtutil

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AdminClaims/AdminSigner are a deliberately separate type from
// Claims/Signer, signed with their own secret - not just a different claim
// shape under the same secret. jwt.ParseWithClaims doesn't error on a
// merely-absent field, it just zero-values it, so a user's own valid token
// would otherwise decode "successfully" against AdminClaims (AdminID ==
// uuid.Nil) if both used the same secret, unless every admin-gated handler
// remembered to reject a nil id. A separate secret makes that whole class
// of mistake impossible at the signature-verification layer instead.
type AdminClaims struct {
	AdminID uuid.UUID `json:"admin_id"`
	jwt.RegisteredClaims
}

type AdminSigner struct {
	secret     []byte
	expiryMins int
}

func NewAdminSigner(secret string, expiryMins int) *AdminSigner {
	return &AdminSigner{secret: []byte(secret), expiryMins: expiryMins}
}

func (s *AdminSigner) Sign(adminID uuid.UUID) (string, error) {
	claims := AdminClaims{
		AdminID: adminID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(s.expiryMins) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *AdminSigner) Parse(tokenString string) (uuid.UUID, error) {
	claims := &AdminClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, errors.New("invalid or expired token")
	}
	return claims.AdminID, nil
}
