package jwt

import (
	"backend/internal/model/entity"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secretKey = "CanvasCloudDefaultJWTSecret" // 生产环境应从配置读取

type Claims struct {
	UserID      uint64 `json:"uid"`
	UserAccount string `json:"acc"`
	UserName    string `json:"name"`
	UserAvatar  string `json:"avatar"`
	UserRole    string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(user *entity.User) (string, error) {
	claims := &Claims{
		UserID:      user.ID,
		UserAccount: user.UserAccount,
		UserName:    user.UserName,
		UserAvatar:  user.UserAvatar,
		UserRole:    user.UserRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func VerifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
