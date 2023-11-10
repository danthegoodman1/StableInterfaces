# StableInterfaces

Consistently routable interface instances in Go without the need for storage or coordination. Inspired by the use case for Cloudflare Durable Objects.

## Connection Handling

### Synchronous `Send()`

The `Send()` does not wait for a response, it launches the `OnRecv()` handler in a goroutine to release closure locks.

If you want to wait for the result of `Send()` (perhaps you want to verify that either the interface or client handled the message), you can simply pass an included `chan error` that you listen for before exiting.

## Alarms

You can optionally enable alarms by passing `WithAlarm()` to `NewInterfaceManager`:

```go
NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
    // ...
}, WithAlarm(AlarmManager))
```

The `AlarmManager` is an interface that implements a few functions for persisting and recovering from restarts. It is expected that you use a consistent, ordered key-value store. Some examples of systems in which you can do this:

1. Postgres/CockroachDB, MySQL
2. Cassandra/ScyllaDB with LWTs or (n/2)+1 consistency level
3. Redis sorted sets with AOF persistence
4. FoundationDB, etcd, and other consistent KV stores

Alarms are managed per-shard, and mostly write to the database (creation and deletion). Reads only occur when a node restarts and needs to read in any previously created alarms.

Unlike DurableObjects, alarms are a bit more capable in StableInterfaces. Each alarm has an `ID` and `Meta` available to it. This means you can make multiple alarms at the same time, which will fire off in (time, ID) order. `Meta` is a `map[string]any`, so you can attach metadata to your alarm to know what it's for. This is a simple durable pattern for background processing.

By default, alarms will be tested for every 150 milliseconds. You can override this with the `WithAlarmCheckInterval()` option. The reason we do this instead of using `time.Timer` is because that creates a goroutine for each alarm. Additionally, alarms are handled sequentially, one at a time. This interval is only used between checks of no active alarms. If an alarm fires, the handler is launched in a goroutine and another alarm is immediately checked for.

It's important to note that when started, every shard will query the AlarmManager for the latest alarms. You may want to introduce some form of rate limiting if you unable to handle the burst of query activity. See https://github.com/danthegoodman1/StableInterfaces/issues/3#issuecomment-1804756669 for more.