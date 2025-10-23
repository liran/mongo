// Package mongo provides client options configuration.
package mongo

import "go.mongodb.org/mongo-driver/mongo/options"

// ClientOptions is an alias for the official MongoDB client options.
// It provides configuration options for MongoDB client connections.
//
// Example:
//
//	db := mongo.NewDatabase(uri, "myapp", func(c *mongo.ClientOptions) {
//	    c.SetMaxPoolSize(100)
//	    c.SetMinPoolSize(10)
//	    c.SetMaxConnIdleTime(30 * time.Second)
//	})
type ClientOptions = options.ClientOptions
