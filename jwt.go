package fit

import (
	"errors"
	"github.com/golang-jwt/jwt"
)

type JwtClaims struct {
	Audience  string `json:"aud,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	Id        string `json:"jti,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	Issuer    string `json:"iss,omitempty"`
	NotBefore int64  `json:"nbf,omitempty"`
	Subject   string `json:"sub,omitempty"`
}

func NewJwtClaims(signingKey string, claims JwtClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims(claims))
	ss, err := token.SignedString([]byte(signingKey))
	return ss, err
}

func Valid(signingKey, t string) (JwtClaims, error) {
	if len(signingKey) == 0 {
		return JwtClaims{}, errors.New("SigningKey is empty")
	}
	sc := &jwt.StandardClaims{}
	token, err := jwt.ParseWithClaims(t, sc, func(token *jwt.Token) (i interface{}, err error) {
		return []byte(signingKey), nil
	})
	if err != nil {
		return JwtClaims{}, err
	}
	if token.Valid == false {
		return JwtClaims{}, errors.New("token is invalid")
	}
	return JwtClaims(*sc), nil
}
