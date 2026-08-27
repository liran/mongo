// Package mongo provides a typed MongoDB SDK built on the official Go driver.
package mongo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"

	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Client wraps the official MongoDB client.
type Client struct {
	*driver.Client
}

// NewClient creates a client from uri and optional official driver settings.
//
// MongoDB clients connect lazily: a successful return means the URI and options
// are valid, not that a server is reachable. Call Ping when readiness is needed.
func NewClient(uri string, configure ...func(*ClientOptions)) (*Client, error) {
	clientOptions := options.Client().ApplyURI(uri)
	for _, apply := range configure {
		apply(clientOptions)
	}

	client, err := driver.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	result := &Client{Client: client}
	return result, nil
}

// Close disconnects the client with a 10-second cleanup timeout.
// It is safe to call Close on a nil client.
func (c *Client) Close() error {
	if c == nil || c.Client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.Disconnect(ctx)
}

// ParseTLSConfig creates a TLS configuration whose root pool contains the PEM
// certificates. It returns an error when pemFile contains no valid certificate.
func ParseTLSConfig(pemFile []byte) (*tls.Config, error) {
	tlsConfig := new(tls.Config)
	tlsConfig.RootCAs = x509.NewCertPool()
	if !tlsConfig.RootCAs.AppendCertsFromPEM(pemFile) {
		return nil, errors.New("failed parsing PEM certificate")
	}
	return tlsConfig, nil
}
