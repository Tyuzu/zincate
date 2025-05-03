package middleware

import (
	"context"
	"fmt"
	"log"
	"naevis/globals"
	"naevis/rdx"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
)

// JWT claims
type Claims struct {
	Username string `json:"username"`
	UserID   string `json:"userId"`
	jwt.RegisteredClaims
}

func Authenticate(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
			http.Error(w, "Invalid token format", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
			return globals.JwtSecret, nil
		})
		fmt.Println(err)
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		fmt.Println("online:" + claims.UserID)
		if err := rdx.SetWithExpiry("online:"+claims.UserID, "", 10*time.Second); err != nil {
			log.Printf("warning: could not set online status for %s: %v", claims.UserID, err)
		}

		// Store UserID in context
		ctx := context.WithValue(r.Context(), globals.UserIDKey, claims.UserID)
		// Pass updated context to the next handler
		next(w, r.WithContext(ctx), ps)
	}
}

func ValidateJWT(tokenString string) (*Claims, error) {
	if tokenString == "" || len(tokenString) < 8 {
		return nil, fmt.Errorf("invalid token")
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
		return globals.JwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}
	return claims, nil
}
