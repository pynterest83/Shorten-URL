package main

import (
	"github.com/gorilla/mux"
)

type Application struct {
	Shortener *URLShortener
	Router    *mux.Router
}

// NewApplication sets up the Application with URLShortener and routes
func NewApplication(shortener *URLShortener) *Application {
	app := &Application{
		Shortener: shortener,
		Router:    mux.NewRouter(),
	}
	app.setupRoutes()
	return app
}

func (app *Application) setupRoutes() {
	app.Router.HandleFunc("/short/{id}", app.Shortener.GetLink).Methods("GET")
	app.Router.HandleFunc("/create", app.Shortener.ShortenURL).Methods("POST")
}
