# StableInterfaces

Consistently routable interface instances in Go without the need for storage or coordination. Inspired by the use case for Cloudflare Durable Objects.

<!-- TOC -->
* [StableInterfaces](#stableinterfaces)
  * [Summary](#summary)
  * [Stable Interfaces (SIs) vs DurableObjects (DOs)](#stable-interfaces-sis-vs-durableobjects-dos)
  * [Hosts](#hosts)
  * [Connection Handling](#connection-handling)
    * [Synchronous `Send()`](#synchronous-send)
  * [Alarms](#alarms)
<!-- TOC -->

## Summary

Stable Interfaces (SIs) provide scaffolding for building consistently-routable instances of Go interfaces across one or more machines. They guarantee a single instance of an interface is running (if you deploy correctly, sacrificing availability for consistency), and all do so without the need for storage, consensus, or coordination

## Stable Interfaces (SIs) vs DurableObjects (DOs)

DOs are fantastic, and cover >90% of use cases. There are some cases where they simply don't work, and this is where SIs excel:

1. Go interfaces, _extreme_ flexibility
2. Max resources per instance is the host's resources
3. Get a full OS underneath
4. Run anywhere (in your api, airgapped, separate service)
5. Connect to existing DB for persistence (all your data in one spot)
6. [Better alarms](#alarms)
7. Use any networking: TCP, UDP, GRPC, HTTP/(1,2,3)
8. Unlimited concurrent connections: DO's have a _hard_ 32k limit (I've hit it in production)
9. Cheaper per-unit cost of runtime and requests at scale

If you can use DOs, you probably should. I've only found a few bugs in them 🙃.

## Hosts

Hosts _must_ be named sequentially (e.g. host-1, host-2, etc.). This is very simple to do in system like Kuberenetes with StatefulSets (pro-tip: use `podManagementPolicy: Parllel` when possible), and aren't too terrible using metadata on systems like fly.io. This is because shards are deterministically assigned to hosts in sequential order (e.g. host-1 gets shard 1, host-2 gets shard 2, and so on in a circle).

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

The `AlarmManager` is an interface that implements a few functions for persisting and recovering from restarts. It is expected that you use a consistent key-value store. Some examples of systems in which you can do this:

1. Postgres/CockroachDB, MySQL
2. Cassandra/ScyllaDB with LWTs or (n/2)+1 consistency level
3. Redis sorted sets with AOF persistence
4. FoundationDB, etcd, and other consistent KV stores

Alarms are managed per-shard, and mostly write to the database (creation and deletion). Reads only occur when a node restarts and needs to read in any previously created alarms.

Unlike DurableObjects, alarms are a bit more capable in StableInterfaces. Each alarm has an `ID` and `Meta` available to it. This means you can make multiple alarms at the same time, which will fire off in (time, ID) order. `Meta` is a `map[string]any`, so you can attach metadata to your alarm to know what it's for. You can also list, update, and cancel alarms. This is a simple durable pattern for background processing.

By default, alarms will be tested for every 150 milliseconds. You can override this with the `WithAlarmCheckInterval()` option. The reason we do this instead of using `time.Timer` is because that creates a goroutine for each alarm. Additionally, alarms are handled sequentially, one at a time. This interval is only used between checks of no active alarms. If an alarm fires, the handler is launched in a goroutine and another alarm is immediately checked for.

It's important to note that when started, every shard will query the AlarmManager for the latest alarms. You may want to introduce some form of rate limiting if you are unable to handle the burst of query activity. See https://github.com/danthegoodman1/StableInterfaces/issues/3#issuecomment-1804756669 for more. It currently loads all pending alarms in memory, so make sure you don't have _billions_ of alarms. Each alarm is lightweight (just a struct in a tree), so the only thing you have to worry about is memory size.