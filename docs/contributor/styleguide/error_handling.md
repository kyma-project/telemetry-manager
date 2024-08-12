# Error Handling

## Reflecting Errors in Status Condition Messages
In Telemetry Manager, it is a common practice to reflect errors in status condition messages. For instance, a LogPipeline might require a client TLS certificate to authenticate against a backend. If the certificate is invalid, the error should be captured and mapped to a corresponding reason/message in the LogPipeline status.

### Defining and Using Sentinel Errors
When an error occurs, the package that returns it (e.g., a TLS validator) should define a sentinel error that the calling code can check. For example:

```go
import "errors"

var ErrCertDecodeFailed = errors.New("failed to decode PEM block containing certificate")
```

### Creating Custom Error Types for Additional Context
If additional context is necessary, define a custom error type. For example, in the following code snippet `CertExpiredError` defines an `Expiry` field that can be used by the calling code:

```go
import (
  "fmt"
  "time"
)

type CertExpiredError struct {
  Expiry time.Time
}

func (err *CertExpiredError) Error() string {
  return fmt.Sprintf("certificate expired on %v", err.Expiry)
}
```

### Mapping Technical Errors to User-Facing Messages
Whether using a sentinel or custom error, the low-level technical error message should be used internally as the Error string. However, this technical error should not be exposed directly in user-facing status conditions. Instead, map the technical error to a user-friendly string in the `internal/conditions` package.

This approach provides several benefits:

* Flexibility: You can add more context to error messages as needed.
* Centralized Messaging: The `internal/conditions` package serves as a catalog of all error messages, offering a comprehensive overview.
* Reviewability: Centralizing messages makes it easier for technical writers to review and refine them for clarity and consistency.
