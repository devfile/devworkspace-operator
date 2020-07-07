package webhook

import (
	"context"
	"fmt"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)


const (
	SecureServiceName = "devworkspace-controller"
	CertConfigMapName = "devworkspace-controller-secure-service"
	CertSecretName    = "devworkspace-controller"
	WebhookServerName = "webhook-server"
	WebhookTLSCertsName = "webhook-tls-certs"
)

var log = logf.Log.WithName("webhook")

func SetupWebhooks(ctx context.Context, cfg *rest.Config) error {

	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}

	client, err := crclient.New(cfg, crclient.Options{})
	if err != nil {
		return fmt.Errorf("failed to create new client: %w", err)
	}

	log.Info("Setting up the secure certs")

	// Set up the certs
	err = SetupWebhookCerts(client, ctx, namespace)
	if err != nil {
		return err
	}

	log.Info("Creating the webhook server deployment")

	// Set up the deployment
	err = CreateWebhookServerDeployment(client, ctx, namespace)
	if err != nil {
		return err
	}

	return nil
}