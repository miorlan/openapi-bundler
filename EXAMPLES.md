# Examples

This document provides examples of how to use `openapi-bundler` as a library.

## Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	bundler "github.com/miorlan/openapi-bundler"
)

func main() {
	ctx := context.Background()

	// Simple bundling
	err := bundler.Bundle(ctx, "input.yaml", "output.yaml")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Bundled successfully!")
}
```

## With Validation

```go
package main

import (
	"context"
	"log"

	bundler "github.com/miorlan/openapi-bundler"
)

func main() {
	ctx := context.Background()

	// Bundle with validation
	err := bundler.BundleWithValidation(ctx, "input.yaml", "output.yaml")
	if err != nil {
		log.Fatal(err)
	}
}
```

## With Custom Options

```go
package main

import (
	"context"
	"log"
	"time"

	bundler "github.com/miorlan/openapi-bundler"
)

func main() {
	ctx := context.Background()

	// Create bundler with custom options
	b := bundler.New(
		bundler.WithValidation(true),
		bundler.WithMaxFileSize(10*1024*1024), // 10MB
		bundler.WithMaxDepth(10),
		bundler.WithHTTPTimeout(60*time.Second),
	)

	err := b.Bundle(ctx, "input.yaml", "output.yaml")
	if err != nil {
		log.Fatal(err)
	}
}
```

## With Context Cancellation

```go
package main

import (
	"context"
	"log"
	"time"

	bundler "github.com/miorlan/openapi-bundler"
)

func main() {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	b := bundler.New()
	err := b.Bundle(ctx, "input.yaml", "output.yaml")
	if err != nil {
		if err == context.DeadlineExceeded {
			log.Fatal("Bundling timed out")
		}
		log.Fatal(err)
	}
}
```

## Error Handling

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	bundler "github.com/miorlan/openapi-bundler"
	"github.com/miorlan/openapi-bundler/internal/domain"
)

func main() {
	ctx := context.Background()
	b := bundler.New()

	err := b.Bundle(ctx, "input.yaml", "output.yaml")
	if err != nil {
		// Check for specific error types
		var circularErr *domain.ErrCircularReference
		var invalidRefErr *domain.ErrInvalidReference

		if errors.As(err, &circularErr) {
			log.Fatalf("Circular reference detected: %s", circularErr.Path)
		}
		if errors.As(err, &invalidRefErr) {
			log.Fatalf("Invalid reference: %s", invalidRefErr.Ref)
		}

		log.Fatal(err)
	}
}
```