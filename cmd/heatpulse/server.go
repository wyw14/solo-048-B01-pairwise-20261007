package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"heatpulse/internal/api"
)

type server struct {
	app  *api.App
	http *http.Server
}

func newServer(configuration config) (*server, error) {
	app, err := api.NewApp(configuration.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("assemble application: %w", err)
	}
	return &server{app: app, http: &http.Server{
		Addr: configuration.Address, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}}, nil
}

func (s *server) run() error {
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *server) shutdown(ctx context.Context) error {
	httpErr := s.http.Shutdown(ctx)
	appErr := s.app.Close(ctx)
	if httpErr != nil {
		return httpErr
	}
	return appErr
}
