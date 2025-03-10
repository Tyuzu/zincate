package auth

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	loginHandler(w, r)
}
func Register(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	registerHandler(w, r)
}
func LogoutUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logoutUserHandler(w, r)
}
func RefreshToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	refreshTokenHandler(w, r)
}
