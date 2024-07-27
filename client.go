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

type Client struct {
	*mongo.Client
}

func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.Disconnect(ctx)
}

func NewClient(connectionURI string, opts ...func(c *ClientOptions)) *Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opt := options.Client().ApplyURI(connectionURI)
	copt := (*ClientOptions)(opt)
	for _, v := range opts {
		v(copt)
	}

	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		log.Fatalln(err)
	}
	return &Client{Client: client}
}

func ParseTLSConfig(pemFile []byte) (*tls.Config, error) {
	tlsConfig := new(tls.Config)
	tlsConfig.RootCAs = x509.NewCertPool()
	ok := tlsConfig.RootCAs.AppendCertsFromPEM(pemFile)
	if !ok {
		return nil, errors.New("failed parsing pem file")
	}
	return tlsConfig, nil
}
