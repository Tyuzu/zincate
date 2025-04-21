package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {

	handleBytes := http.HandlerFunc(imageAsBytesStream)
	handleAttachment := http.HandlerFunc(imageAsAttachment)

	http.Handle("/static", handleBytes)
	http.Handle("/attachment", handleAttachment)

	fmt.Println("Server started at port 8080")
	http.ListenAndServe(":8080", nil)
}

func imageAsBytesStream(w http.ResponseWriter, r *http.Request) {

	buf, err := os.ReadFile("./merchpic/jisoo.gif")

	if err != nil {

		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(buf)
}

func imageAsAttachment(w http.ResponseWriter, r *http.Request) {

	buf, err := os.ReadFile("./merchpic/jisoo.gif")

	if err != nil {

		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", `attachment;filename="sid.png"`)

	w.Write(buf)
}

// router.ServeFiles("/static/postpic/*filepath", http.Dir("static/postpic"))
// router.ServeFiles("/static/merchpic/*filepath", http.Dir("static/merchpic"))
// router.ServeFiles("/static/menupic/*filepath", http.Dir("static/menupic"))
// router.ServeFiles("/static/uploads/*filepath", http.Dir("static/uploads"))
// router.ServeFiles("/static/placepic/*filepath", http.Dir("static/placepic"))
// router.ServeFiles("/static/businesspic/*filepath", http.Dir("static/eventpic"))
// router.ServeFiles("/static/userpic/*filepath", http.Dir("static/userpic"))
// router.ServeFiles("/static/eventpic/*filepath", http.Dir("static/eventpic"))
