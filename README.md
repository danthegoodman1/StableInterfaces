# StableInterfaces

Consistently routable interface instances in Go without the need for storage or coordination. Inspired by the use case for Cloudflare Durable Objects.

## Connection Handling

### Synchronous `Send()`

The `Send()` does not wait for a response, it launches the `OnRecv()` handler in a goroutine to release closure locks.

If you want to wait for the result of `Send()` (perhaps you want to verify that either the interface or client handled the message), you can simply pass an included `chan error` that you listen for before exiting.