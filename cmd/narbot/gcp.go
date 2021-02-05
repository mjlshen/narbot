package main

import (
	"fmt"

	"cloud.google.com/go/compute/metadata"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"golang.org/x/net/context"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// Require compute.projects.get
func getProjectId() (string, error) {
	if metadata.OnGCE() {
		return metadata.ProjectID()
	}

	const url = "https://cloud.google.com/run/docs/reference/container-contract#metadata-server"
	return "", fmt.Errorf("Could not determine Project ID from metadata server. See %v for more information.", url)
}

// accessSecretVersion accesses the payload for the latest version of a
// given secret from GCP Secret Manager in a project
func getValueSecretManager(projectId string, secretName string) (string, error) {
	name := "projects/" + projectId + "/secrets/" + secretName + "/versions/latest"

	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("Failed to create secretmanager client: %v", err)
	}

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Failed to access secret version: %v", err)
	}

	return string(result.Payload.Data), nil
}
