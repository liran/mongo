// Package mongo provides a high-level, type-safe wrapper around the official MongoDB Go driver.
// It offers simplified APIs for common database operations with automatic index management,
// transaction support, and enhanced error handling.
//
// Key Features:
//   - Simplified CRUD operations with automatic ID handling
//   - Automatic index management using struct tags
//   - Transaction support for both single and multi-document operations
//   - Type-safe data conversion utilities
//   - Built-in pagination and cursor-based iteration
//   - Comprehensive error handling with custom error types
//
// Example:
//
//	db := mongo.NewDatabase("mongodb://localhost:27017", "myapp")
//	defer db.Close()
//
//	user := &User{ID: "user123", Name: "John"}
//	err := db.Set(user)
//
// https://www.mongodb.com/docs/drivers/go/current/quick-start/
package mongo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Client wraps the official MongoDB client with enhanced functionality.
// It provides a simplified interface for common database operations.
type Client struct {
	*mongo.Client
}

// Close gracefully closes the MongoDB client connection.
// It uses a 10-second timeout to ensure proper cleanup.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.Disconnect(ctx)
}

// NewClient creates a new MongoDB client with the given connection URI.
// Optional client options can be provided to customize the connection behavior.
//
// Example:
//
//	client := mongo.NewClient("mongodb://localhost:27017")
//	client := mongo.NewClient(uri, func(c *mongo.ClientOptions) {
//	    c.SetMaxPoolSize(100)
//	})
func NewClient(connectionURI string, opts ...func(c *ClientOptions)) *Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opt := options.Client().ApplyURI(connectionURI)
	for _, v := range opts {
		v(opt)
	}

	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		log.Fatalln(err)
	}
	return &Client{Client: client}
}

// ParseTLSConfig creates a TLS configuration from PEM certificate data.
// This is useful for connecting to MongoDB instances with SSL/TLS encryption.
//
// Example:
//
//	tlsConfig, err := mongo.ParseTLSConfig(pemFileBytes)
//	if err != nil {
//	    log.Fatal(err)
//	}
func ParseTLSConfig(pemFile []byte) (*tls.Config, error) {
	tlsConfig := new(tls.Config)
	tlsConfig.RootCAs = x509.NewCertPool()
	ok := tlsConfig.RootCAs.AppendCertsFromPEM(pemFile)
	if !ok {
		return nil, errors.New("failed parsing pem file")
	}
	return tlsConfig, nil
}
