# MongoDB Go Driver Wrapper

A high-level, type-safe wrapper around the official MongoDB Go driver that provides a simplified API for common database operations with automatic index management and transaction support.

## Features

- **Simplified API**: Clean, intuitive methods for CRUD operations
- **Automatic Index Management**: Define indexes using struct tags
- **Transaction Support**: Built-in support for both single-document and multi-document transactions
- **Type Safety**: Generic functions for type-safe data conversion
- **Error Handling**: Comprehensive error handling with custom error types
- **Pagination**: Built-in pagination support
- **Bulk Operations**: Support for bulk updates and operations
- **TLS Support**: Built-in TLS configuration utilities

## Installation

```bash
go get github.com/liran/mongo
```

## Quick Start

### Basic Connection

```go
package main

import (
    "context"
    "github.com/liran/mongo"
)

func main() {
    // Create a new database connection
    db := mongo.NewDatabase("mongodb://localhost:27017", "myapp")
    defer db.Close()
    
    // Your application code here
}
```

### Define Models with Indexes

```go
type User struct {
    ID       string     `bson:"_id"`
    Name     string     `bson:"name" db:"unique"`
    Email    string     `bson:"email" db:"unique"`
    Age      int64      `bson:"age" db:"index"`
    CreatedAt *time.Time `bson:"created_at,omitempty" db:"index"`
    
    // Compound unique index
    Domain   string     `bson:"domain" db:"unique,group=user_domain_region"`
    Region   string     `bson:"region" db:"unique,group=user_domain_region"`
}

type Job struct {
    TaskID string `bson:"task_id" db:"unique,group=job_task_url"`
    URL    string `bson:"url" db:"unique,group=job_task_url"`
    Status string `bson:"status" db:"index"`
    Owner  string `bson:"owner" db:"unique"`
}
```

### Create Indexes

```go
ctx := context.Background()

// Create indexes for your models
err := db.Indexes(ctx, &User{}, &Job{})
if err != nil {
    log.Fatal(err)
}
```

## Database Operations

### Basic CRUD Operations

```go
// Create/Update (Upsert)
user := &User{
    ID:    "user123",
    Name:  "John Doe",
    Email: "john@example.com",
    Age:   30,
}

err := db.Set(user)
if err != nil {
    log.Fatal(err)
}

// Read
var foundUser User
err = db.Unmarshal("user123", &foundUser)
if err != nil {
    if errors.Is(err, mongo.ErrRecordNotFound) {
        log.Println("User not found")
    } else {
        log.Fatal(err)
    }
}

// Update
user.Age = 31
newRecord, err := db.Update(user)
if err != nil {
    log.Fatal(err)
}

// Delete
err = db.Delete(&User{}, "user123")
if err != nil {
    log.Fatal(err)
}
```

### Advanced Queries

```go
// Find first record with filter and sort
record, err := db.First(&User{}, 
    mongo.Map().Set("age", mongo.Map().Set("$gte", 18)),
    mongo.Map().Set("created_at", -1),
)
if err != nil {
    log.Fatal(err)
}

// Count documents
count, err := db.Count(&User{}, mongo.Map().Set("age", mongo.Map().Set("$gte", 18)))
if err != nil {
    log.Fatal(err)
}

// Check if document exists
exists, err := db.Has(&User{}, "user123")
if err != nil {
    log.Fatal(err)
}
```

### Pagination

```go
// Get paginated results
total, list, err := db.Pagination(
    &User{},                    // model
    mongo.Map().Set("age", mongo.Map().Set("$gte", 18)), // filter
    mongo.Map().Set("created_at", -1), // sort
    1,                          // page
    10,                         // page size
)
if err != nil {
    log.Fatal(err)
}

// Convert to typed entities
users := mongo.ToEntities[User](list)
for _, user := range users {
    fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
}
```

### List with Callback

```go
// Iterate through all users
err = db.List(context.Background(), &User{}, mongo.Map(), func(m mongo.M) (bool, error) {
    user := mongo.ToEntity[User](m)
    fmt.Printf("Processing user: %s\n", user.Name)
    return true, nil // return false to stop iteration
})
if err != nil {
    log.Fatal(err)
}
```

## Transaction Support

### Single Document Transactions

```go
err := db.Txn(ctx, func(txn *mongo.Txn) error {
    // All operations in this function are atomic
    user := &User{ID: "user123", Name: "John", Age: 30}
    err := txn.Model(user).Set(user)
    if err != nil {
        return err
    }
    
    // Update the user
    user.Age = 31
    _, err = txn.Model(user).Update(user)
    return err
}, false) // false = single document transaction
```

### Multi-Document Transactions

```go
err := db.Txn(ctx, func(txn *mongo.Txn) error {
    // Create user
    user := &User{ID: "user123", Name: "John", Age: 30}
    err := txn.Model(user).Set(user)
    if err != nil {
        return err
    }
    
    // Create job
    job := &Job{TaskID: "task123", URL: "http://example.com", Status: "pending", Owner: "user123"}
    err = txn.Model(job).Set(job)
    return err
}, true) // true = multi-document transaction
```

## Advanced Operations

### Increment Operations

```go
err := db.Txn(ctx, func(txn *mongo.Txn) error {
    // Increment/decrement fields
    return txn.Model(&User{}).Inc("user123", 
        mongo.Map().Set("age", 1).Set("login_count", 1))
}, true)
```

### Bulk Updates

```go
err := db.Txn(ctx, func(txn *mongo.Txn) error {
    filter := mongo.Map().Set("status", "pending")
    update := mongo.Map().Set("status", "completed").Set("completed_at", time.Now())
    
    count, err := txn.Model(&Job{}).UpdateMany(filter, update)
    fmt.Printf("Updated %d jobs\n", count)
    return err
}, true)
```

### Cursor-based Iteration

```go
err := db.Txn(ctx, func(txn *mongo.Txn) error {
    return txn.Model(&User{}).ListByCursor(
        mongo.Map().Set("age", mongo.Map().Set("$gte", 18)), // filter
        true,  // descending order
        100,   // batch size
        func(m mongo.M) (bool, error) {
            user := mongo.ToEntity[User](m)
            fmt.Printf("User: %s\n", user.Name)
            return true, nil
        },
    )
}, false)
```

## Index Management

### Index Tags

The package supports several index-related tags:

- `db:"index"` - Create a regular index
- `db:"unique"` - Create a unique index
- `db:"unique,group=group_name"` - Create a compound unique index
- `db:"index,group=group_name"` - Create a compound index

### Examples

```go
type Product struct {
    ID          string `bson:"_id"`
    Name        string `bson:"name" db:"index"`
    SKU         string `bson:"sku" db:"unique"`
    Category    string `bson:"category" db:"index,group=category_price"`
    Price       int64  `bson:"price" db:"index,group=category_price"`
    CreatedAt   *time.Time `bson:"created_at,omitempty" db:"index"`
}
```

This will create:
- Single field indexes: `name`, `sku` (unique), `created_at`
- Compound index: `category_price` on fields `category` and `price`

## Error Handling

The package provides custom error types:

```go
var (
    ErrInvalidModelName = errors.New("invalid model name")
    ErrNoID             = errors.New("no id. not found primary key from model")
    ErrRecordNotFound   = errors.New("record not found")
    ErrDuplicateKey     = errors.New("duplicate key error")
)
```

## Utility Functions

### Type Conversion

```go
// Convert map to typed struct
user := mongo.ToEntity[User](mongoMap)

// Convert slice of maps to slice of typed structs
users := mongo.ToEntities[User](mapSlice)
```

### ID Generation

```go
// Generate sequential ID
id := mongo.SequentialID()

// Generate random number in range
num := mongo.RandInRange(1, 100)
```

### Map Operations

```go
// Create and manipulate maps
filter := mongo.Map().
    Set("age", mongo.Map().Set("$gte", 18)).
    Set("status", "active")

// Get value from map
if age, ok := filter.Get("age"); ok {
    // use age
}

// Delete key from map
filter.Del("status")
```

## TLS Configuration

```go
// Parse TLS config from PEM file
tlsConfig, err := mongo.ParseTLSConfig(pemFileBytes)
if err != nil {
    log.Fatal(err)
}

// Use with client options
db := mongo.NewDatabase(uri, "myapp", func(c *mongo.ClientOptions) {
    c.TLSConfig = tlsConfig
})
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
