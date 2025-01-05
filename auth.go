package main

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
)

// JWT claims
type Claims struct {
	Username string `json:"username"`
	UserID   string `json:"userId"`
	jwt.RegisteredClaims
}

func authenticate(next httprouter.Handle) httprouter.Handle {
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
		token, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Store UserID in context
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		// Pass updated context to the next handler
		next(w, r.WithContext(ctx), ps)
	}
}

func login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	loginHandler(w, r)
}
func register(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	registerHandler(w, r)
}
func logoutUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logoutUserHandler(w, r)
}
func refreshToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	refreshTokenHandler(w, r)
}
