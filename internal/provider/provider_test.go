package provider

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

// Acceptance tests run against a real SolidPing server (see README, "Developing"):
//
//	SOLIDPING_ENDPOINT, SOLIDPING_ORG  the server and organization to use
//	SOLIDPING_TOKEN                    a personal access token, or
//	SOLIDPING_ACC_EMAIL / _PASSWORD    credentials the tests mint a token with
const (
	envAccEmail    = "SOLIDPING_ACC_EMAIL"
	envAccPassword = "SOLIDPING_ACC_PASSWORD" //nolint:gosec // name of an environment variable
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"solidping": providerserver.NewProtocol6WithError(New("test")()),
}

var (
	accSetup    sync.Once
	accSetupErr error
	accVersion  string
)

func testAccClient() *client.Client {
	return client.New(os.Getenv(envEndpoint), os.Getenv(envOrg), os.Getenv(envToken), "terraform-provider-solidping/test")
}

func testAccPreCheck(t *testing.T) {
	t.Helper()

	for _, name := range []string{envEndpoint, envOrg} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s must be set for acceptance tests", name)
		}
	}

	accSetup.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		api := testAccClient()

		if os.Getenv(envToken) == "" {
			email, password := os.Getenv(envAccEmail), os.Getenv(envAccPassword)
			if email == "" || password == "" {
				t.Fatalf("set %s, or %s and %s, for acceptance tests", envToken, envAccEmail, envAccPassword)
			}

			session, err := api.Login(ctx, email, password)
			if err != nil {
				accSetupErr = err

				return
			}

			sessionClient := client.New(os.Getenv(envEndpoint), os.Getenv(envOrg), session, "terraform-provider-solidping/test")

			token, err := sessionClient.CreateToken(ctx, "terraform-acceptance-tests")
			if err != nil {
				accSetupErr = err

				return
			}

			_ = os.Setenv(envToken, token)
		}

		version, err := api.GetVersion(ctx)
		if err != nil {
			accSetupErr = err

			return
		}

		accVersion = version.Version
	})

	if accSetupErr != nil {
		t.Fatalf("acceptance test setup: %v", accSetupErr)
	}
}

// serverAtLeast tells whether the server under test is at least major.minor.
// A development build ("dev") is taken as the latest.
func serverAtLeast(major, minor int) bool {
	parts := strings.SplitN(strings.TrimPrefix(accVersion, "v"), ".", 3)
	if len(parts) < 2 {
		return true
	}

	gotMajor, err1 := strconv.Atoi(parts[0])
	gotMinor, err2 := strconv.Atoi(parts[1])

	if err1 != nil || err2 != nil {
		return true
	}

	return gotMajor > major || (gotMajor == major && gotMinor >= minor)
}

const testAccProviderConfig = `
provider "solidping" {}
`
