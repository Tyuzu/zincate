package routes

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func AddStaticRoutes(router *httprouter.Router) {
	router.ServeFiles("/static/postpic/*filepath", http.Dir("static/postpic"))
	router.ServeFiles("/static/merchpic/*filepath", http.Dir("static/merchpic"))
	router.ServeFiles("/static/menupic/*filepath", http.Dir("static/menupic"))
	router.ServeFiles("/static/uploads/*filepath", http.Dir("static/uploads"))
	router.ServeFiles("/static/placepic/*filepath", http.Dir("static/placepic"))
	router.ServeFiles("/static/businesspic/*filepath", http.Dir("static/eventpic"))
	router.ServeFiles("/static/userpic/*filepath", http.Dir("static/userpic"))
	router.ServeFiles("/static/eventpic/*filepath", http.Dir("static/eventpic"))
	router.ServeFiles("/static/artistpic/*filepath", http.Dir("static/artistpic"))
	router.ServeFiles("/static/cartoonpic/*filepath", http.Dir("static/cartoonpic"))
}
