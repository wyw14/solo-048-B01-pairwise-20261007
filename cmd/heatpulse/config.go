package main

import "os"

type config struct {
	Address       string
	DataDirectory string
}

func loadConfig() config {
	address := os.Getenv("HEATPULSE_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	dataDirectory := os.Getenv("HEATPULSE_DATA")
	if dataDirectory == "" {
		dataDirectory = "data"
	}
	return config{Address: address, DataDirectory: dataDirectory}
}
