package main

import (
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	fqdn string
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	//

	fqdn = GetRandomString(20) + "." + zone

	// Since the framework does not load the secret yaml, we use the workaround from
	d, err := os.ReadFile("testdata/config.json")
	if err != nil {
		log.Fatal(err)
	}

	duration, err := time.ParseDuration("5m")
	if err != nil {
		log.Fatal(err)
	}

	fixture := acmetest.NewFixture(&stratoDNSProviderSolver{},
		acmetest.SetResolvedZone(zone),
		acmetest.SetResolvedFQDN(fqdn),
		acmetest.SetStrict(true),
		acmetest.SetPropagationLimit(duration),
		acmetest.SetManifestPath("testdata/strato-secret.yaml"),
		acmetest.SetConfig(&extapi.JSON{
			Raw: d,
		}),
	)
	fixture.RunConformance(t)

}

func GetRandomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
