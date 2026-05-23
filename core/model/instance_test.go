package model

import (
	"testing"

	"infini.sh/framework/core/config"
)

func TestResolveManagedInstanceEndpoint(t *testing.T) {
	t.Run("prefer api endpoint when api is enabled", func(t *testing.T) {
		apiConfig := config.APIConfig{Enabled: true}
		apiConfig.NetworkConfig.Publish = "127.0.0.1:2900"
		webConfig := config.WebAppConfig{Enabled: true}
		webConfig.NetworkConfig.Publish = "127.0.0.1:8080"

		endpoint := resolveManagedInstanceEndpoint(apiConfig, webConfig)
		if endpoint != "http://127.0.0.1:2900" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
	})

	t.Run("fallback to web endpoint when api is disabled", func(t *testing.T) {
		apiConfig := config.APIConfig{Enabled: false}
		apiConfig.NetworkConfig.Publish = "127.0.0.1:2900"
		webConfig := config.WebAppConfig{Enabled: true}
		webConfig.NetworkConfig.Publish = "127.0.0.1:8080"

		endpoint := resolveManagedInstanceEndpoint(apiConfig, webConfig)
		if endpoint != "http://127.0.0.1:8080" {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
	})
}

func TestBuildManagedInstanceServices(t *testing.T) {
	apiConfig := config.APIConfig{Enabled: true}
	apiConfig.NetworkConfig.Publish = "127.0.0.1:2900"
	webConfig := config.WebAppConfig{Enabled: true}
	webConfig.NetworkConfig.Publish = "127.0.0.1:8080"

	services := buildManagedInstanceServices(apiConfig, webConfig)
	if len(services) != 2 {
		t.Fatalf("unexpected service count: %#v", services)
	}
	if services[0].Name != "api" || services[0].Endpoint != "http://127.0.0.1:2900" {
		t.Fatalf("unexpected api service: %#v", services[0])
	}
	if services[1].Name != "web" || services[1].Endpoint != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected web service: %#v", services[1])
	}
}
