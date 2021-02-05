package main

import (
	"io/ioutil"
	"log"
	"narbot/pkg/endpoint"
	"net/http"
	"os"
)

type NarbotConfig struct {
	Endpoints     []endpoint.Endpoint
	SigningSecret string
}

func loadConfig() (NarbotConfig, error) {
	var (
		c      NarbotConfig
		p      string
		exists bool
		err    error
	)

	c.SigningSecret, exists = os.LookupEnv("NARBOT_SIGNING_SECRET")
	// Try to load narbot-signing-secret from GCP Secrets Manager
	if !exists {
		p, err = getProjectId()
		if err != nil {
			return c, err
		}

		c.SigningSecret, err = getValueSecretManager(p, "narbot-signing-secret")
		if err != nil {
			return c, err
		}
	}

	file, exists := os.LookupEnv("NARBOT_ENDPOINTS_CONFIG")
	if !exists {
		file = "config.json"
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return c, err
	}
	c.Endpoints, err = endpoint.ReadEndpoints(data)
	if err != nil {
		return c, err
	}

	return c, err
}

func main() {
	log.Println("[INFO] narbot starting")

	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/narbot", config.slashCommandHandler)
	log.Println("[INFO] Server listening")
	http.ListenAndServe(":8080", nil)
}
