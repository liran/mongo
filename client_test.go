package mongo

import (
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/xoptions"
)

func TestNewClientAndOpen(t *testing.T) {
	t.Run("client", func(t *testing.T) {
		deployment := drivertest.NewMockDeployment()
		configure := func(clientOptions *ClientOptions) {
			err := xoptions.SetInternalClientOptions(clientOptions, "deployment", deployment)
			require.NoError(t, err)
		}

		client, err := NewClient("mongodb://localhost", configure)
		require.NoError(t, err)
		require.NotNil(t, client.Client)
		require.NoError(t, client.Close())
	})

	t.Run("database", func(t *testing.T) {
		deployment := drivertest.NewMockDeployment()
		configure := func(clientOptions *ClientOptions) {
			err := xoptions.SetInternalClientOptions(clientOptions, "deployment", deployment)
			require.NoError(t, err)
		}

		database, err := Open("mongodb://localhost", "unit", configure)
		require.NoError(t, err)
		require.Equal(t, "unit", database.Name())
		require.NoError(t, database.Close())
		require.NoError(t, database.Close())
	})

	t.Run("invalid URI", func(t *testing.T) {
		_, err := NewClient("://invalid")
		require.Error(t, err)
	})

	t.Run("nil close", func(t *testing.T) {
		var client *Client
		require.NoError(t, client.Close())
		var database *Database
		require.NoError(t, database.Close())
	})
}

func TestTLSConfigFromPEM(t *testing.T) {
	t.Run("valid certificate", func(t *testing.T) {
		certificate := makeTestCertificate(t)
		config, err := TLSConfigFromPEM(certificate)
		require.NoError(t, err)
		require.NotNil(t, config.RootCAs)
	})

	t.Run("invalid certificate", func(t *testing.T) {
		_, err := TLSConfigFromPEM([]byte("not a certificate"))
		require.Error(t, err)
	})
}

func makeTestCertificate(t *testing.T) []byte {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(cryptorand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "mongo-sdk-test"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Minute),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(cryptorand.Reader, template, template, publicKey, privateKey)
	require.NoError(t, err)
	block := &pem.Block{Type: "CERTIFICATE", Bytes: der}
	return pem.EncodeToMemory(block)
}
