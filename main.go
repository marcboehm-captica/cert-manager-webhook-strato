package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/fl0eb/go-strato"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our strato DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&stratoDNSProviderSolver{},
	)
}

// stratoDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/cert-manager/cert-manager/pkg/acme/webhook.Solver`
// interface.
type stratoDNSProviderSolver struct {
	// If a Kubernetes 'clientset' is needed, you must:
	// 1. uncomment the additional `client` field in this structure below
	// 2. uncomment the "k8s.io/client-go/kubernetes" import at the top of the file
	// 3. uncomment the relevant code in the Initialize method below
	// 4. ensure your webhook's service account has the required RBAC role
	//    assigned to it for interacting with the Kubernetes APIs you need.
	client *kubernetes.Clientset
	sync.RWMutex
}

// stratoDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type stratoDNSProviderConfig struct {
	// Change the two fields below according to the format of the configuration
	// to be decoded.
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.

	//Email           string `json:"email"`
	//APIKeySecretRef v1alpha1.SecretKeySelector `json:"apiKeySecretRef"`
	SecretRef string `json:"secretName"`
	API       string `json:"api"`
	Domain    string `json:"domain"`
	Order     string `json:"order"`
}

// stratoDNSProviderCredentials holds the identity and password values.
type stratoDNSProviderCredentials struct {
	Identity string
	Password string
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *stratoDNSProviderSolver) Name() string {
	return "strato"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *stratoDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	klog.V(2).Infof("Executing Strato Webhook Present with arguments: namespace=%s, zone=%s, fqdn=%s",
		ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	return processRecord(c, ch, true)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *stratoDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	klog.V(2).Infof("Executing Strato Webhook Cleanup with arguments: namespace=%s, zone=%s, fqdn=%s",
		ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	return processRecord(c, ch, false)
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *stratoDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	k8sClient, err := kubernetes.NewForConfig(kubeClientConfig)
	klog.V(6).Infof("Input variable stopCh is %d length", len(stopCh))
	if err != nil {
		return err
	}
	c.client = k8sClient
	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (stratoDNSProviderConfig, error) {
	cfg := stratoDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}
	klog.V(6).Infof("Decoded configuration %v", cfg)
	return cfg, nil
}

// loadSecretConfig fetches the secret configuration from Kubernetes and returns credentials.
func loadSecretConfig(client *kubernetes.Clientset, secretName, namespace string) (stratoDNSProviderCredentials, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return stratoDNSProviderCredentials{}, fmt.Errorf("error fetching secret %s/%s: %v", namespace, secretName, err)
	}
	credentials := stratoDNSProviderCredentials{
		Identity: string(secret.Data["identity"]),
		Password: string(secret.Data["password"]),
	}
	klog.V(6).Infof("Decoded credentials")
	return credentials, nil
}

func printDNSConfig(header string, config strato.DNSConfig) {
	klog.V(6).Infof("%s", header)
	klog.V(6).Infof("DMARC Type: %s", config.DMARCType)
	klog.V(6).Infof("SPF Type: %s", config.SPFType)
	klog.V(6).Infof("DNS records:")
	for _, record := range config.Records {
		klog.V(6).Infof("%s:%s=%s", record.Type, record.Prefix, record.Value)
	}
}

func contains(records []strato.DNSRecord, record strato.DNSRecord) bool {
	for _, entry := range records {
		if entry.Type == record.Type && entry.Prefix == record.Prefix && entry.Value == record.Value {
			return true
		}
	}
	return false
}

func prevalidate(cfg stratoDNSProviderConfig, ch *v1alpha1.ChallengeRequest) error {
	if ch.ResolvedFQDN == "" || ch.ResolvedZone == "" {
		return fmt.Errorf("resolved FQDN or resolved zone is empty")
	}

	if ch.ResolvedZone != cfg.Domain+"." && !strings.HasSuffix(ch.ResolvedZone, cfg.Domain+".") {
		return fmt.Errorf("resolved zone '%s' does not match configured domain '%s'", ch.ResolvedZone, cfg.Domain)
	}

	if !strings.HasSuffix(ch.ResolvedFQDN, "."+ch.ResolvedZone) {
		return fmt.Errorf("resolved FQDN '%s' does not end with resolved zone '%s'", ch.ResolvedFQDN, ch.ResolvedZone)
	}

	return nil
}

func processRecord(c *stratoDNSProviderSolver, ch *v1alpha1.ChallengeRequest, add bool) error {

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	secretCfg, err := loadSecretConfig(c.client, cfg.SecretRef, ch.ResourceNamespace)
	if err != nil {
		return err
	}

	err = prevalidate(cfg, ch)
	if err != nil {
		return err
	}

	record := strato.DNSRecord{
		Type:   "TXT",
		Prefix: strings.ToLower(strings.TrimSuffix(ch.ResolvedFQDN, "."+cfg.Domain+".")),
		Value:  ch.Key,
	}

	klog.V(6).Infof("Initializing Strato client for Order: %s and Domain: %s", cfg.Order, cfg.Domain)
	client, err := strato.NewStratoClient(cfg.API, secretCfg.Identity, secretCfg.Password, cfg.Order, cfg.Domain)
	if err != nil {
		return err
	}

	// The webhook should not add / remove anything in the background after we read the current configuration until we are done.
	c.Lock()
	err = updateStrato(client, record, add)
	if err != nil {
		c.Unlock()
		return err
	}
	c.Unlock()
	updatedDNSConfig, err := client.GetDNSConfiguration()
	if err != nil {
		return err
	}
	printDNSConfig("Updated DNS config:", updatedDNSConfig)
	if add {
		if !contains(updatedDNSConfig.Records, record) {
			return fmt.Errorf("failed to add new record")
		}
	} else {
		if contains(updatedDNSConfig.Records, record) {
			return fmt.Errorf("failed to remove record")
		}
	}

	klog.V(2).Infof("TXT record %s=%s processed successfully", record.Prefix, record.Value)
	return nil
}

func updateStrato(client *strato.StratoClient, record strato.DNSRecord, add bool) error {
	klog.V(6).Infof("Fetching current configuration")
	oldDNSConfig, err := client.GetDNSConfiguration()
	if err != nil {
		return err
	}
	printDNSConfig("Previous DNS config:", oldDNSConfig)
	newDNSConfig := oldDNSConfig
	if add {
		klog.V(2).Infof("Adding TXT Record: %s=%s", record.Prefix, record.Value)

		if contains(oldDNSConfig.Records, record) {
			klog.V(2).Infof("TXT record %s=%s already exists. Nothing to do.", record.Prefix, record.Value)
			return nil
		}
		newDNSConfig.Records = append(newDNSConfig.Records, record)
	} else {
		klog.V(2).Infof("Cleaning up TXT Record: %s=%s", record.Prefix, record.Value)

		newDNSConfig.Records = make([]strato.DNSRecord, 0)
		for _, entry := range oldDNSConfig.Records {
			if entry.Type != record.Type || entry.Prefix != record.Prefix || entry.Value != record.Value {
				newDNSConfig.Records = append(newDNSConfig.Records, entry)
			}
		}
		if len(newDNSConfig.Records) == len(oldDNSConfig.Records) {
			klog.V(2).Infof("TXT record %s=%s does not exists. Nothing to do.", record.Prefix, record.Value)
			return nil
		}
	}
	err = client.SetDNSConfiguration(newDNSConfig)
	if err != nil {
		return err
	}
	klog.V(6).Infof("Request to process TXT record %s=%s sent", record.Prefix, record.Value)
	return nil
}
