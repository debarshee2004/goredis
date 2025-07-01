# Goredis - Creating a Redis database using Go-land

A lightweight Redis clone built in Go, supporting core Redis commands, memory management, protocol compliance (RESP), and client-server architecture.

## ⚙️ How the App Works (In Depth)

The GoRedis application operates as a lightweight in-memory key-value database server, mimicking the behavior of Redis using the Go programming language. At its core, it establishes a TCP server using Go’s `net` package, which listens on a configurable port—by default, `:5555`. When a client (such as `redis-cli`) connects, the server spawns a dedicated goroutine to handle all communications with that client, ensuring that connections are managed concurrently and efficiently.

Client commands are sent using the RESP (Redis Serialization Protocol), a text-based protocol designed for simplicity and speed. GoRedis utilizes the `tidwall/resp` library to parse incoming RESP-formatted messages into Go structs and to serialize outgoing responses accordingly. For example, the command `SET name John` is internally interpreted as a RESP array and decoded into individual components for command dispatching.

Command handling in GoRedis is built using the Command Pattern. Each Redis command is defined as a struct that implements a common interface with an `Execute(storage *Storage)` method. This pattern ensures modularity and allows for easy extension. Parsing logic resides in the `peer.go` file, where RESP arrays are validated and mapped to their respective command structs (e.g., `SetCommand`, `GetCommand`, `IncrByCommand`). Execution logic for each command resides in `commands.go`.

All data is stored in-memory using the `Storage` struct, which contains maps for key-value pairs, expiration data, and numeric counters. These maps are concurrency-safe and guarded using Go's `sync.RWMutex`. The `Storage` component is used by all command executions and is responsible for reading, writing, appending, and modifying data.

GoRedis also supports optional TTL (Time To Live) expiration for keys. The `SET` command can include an expiration clause using the `EX` keyword to set the number of seconds a key should remain valid. When the key is accessed after its TTL has expired, it is automatically purged—a method known as lazy expiration. GoRedis provides implementations for basic string operations, atomic counters, batch operations (like `MSET`/`MGET`), and administrative commands such as `FLUSHALL`, `PING`, `HELLO`, and `CLIENT`.

### 🧠 Memory Management (In Depth)

Memory management in GoRedis revolves around an in-memory storage model with concurrency and expiration control built into the `Storage` struct. This struct maintains three primary internal maps: one for general key-value data (`map[string][]byte`), one for expiration times (`map[string]time.Time`), and one for numeric counters (`map[string]int64`). These maps are safeguarded using Go’s read-write mutex (`sync.RWMutex`), which allows multiple readers to operate in parallel but restricts writes to one thread at a time, ensuring data integrity.

<!-- When a key-value pair is stored using the `SET` command, it is inserted into the `data` map, and any previous expiration is cleared. If a TTL is specified (using `EX`), an expiration time is calculated and stored in the `expiry` map. Each time a key is accessed—whether via `GET`, `EXISTS`, or any other read command—the application checks the expiration map to see if the key has expired. If it has, the key is immediately deleted from all internal maps. This strategy, known as lazy expiration, avoids the overhead of a background thread and simplifies memory control.

Atomicity is crucial for numeric operations like `INCR`, `DECR`, `INCRBY`, and `DECRBY`. These operations are implemented in such a way that they parse and store integer values safely, leveraging the `counters` map for fast future access. Operations are always performed under a write lock to maintain atomic guarantees, especially in concurrent environments where multiple clients may modify the same keys.

It’s important to note that GoRedis is a fully in-memory solution. It does not persist data to disk, meaning that all data is lost on restart. This behavior mirrors the ephemeral caching use-case of Redis. However, users can manually reset the memory space at runtime using the `FLUSHALL` command, which clears all stored data and resets all maps.-->

### 📡 Protocol Commands & RESP Support (In Depth)

The GoRedis server fully embraces the RESP (Redis Serialization Protocol), which is used by Redis for communication between clients and the server. RESP is a lightweight and human-readable protocol that encodes simple strings, errors, integers, bulk strings, arrays, and maps using specific prefixes such as `+` for simple strings, `-` for errors, `:` for integers, `$` for bulk strings, `*` for arrays, and `%` for maps. For example, a successful `SET` command might return `+OK\r\n`, while a `GET` on a missing key would return `$-1\r\n`, indicating a null.

<!-- Commands are received over TCP and parsed into RESP values using the `tidwall/resp` reader. The parsed data is then processed in `peer.go`, which interprets the command name and its arguments, and returns a typed `Command` struct. This struct is sent to the server’s message loop for execution, where it interacts with the `Storage` layer and returns a result, which is encoded back into RESP format and sent to the client.

GoRedis supports a broad subset of Redis commands. For string operations, commands like `SET`, `GET`, `DEL`, `EXISTS`, `GETSET`, `APPEND`, `STRLEN`, `GETRANGE`, and `SETRANGE` are implemented to manage string data. Numeric operations include `INCR`, `DECR`, `INCRBY`, and `DECRBY`, allowing for efficient and atomic manipulation of integers stored as strings. Batch operations such as `MSET` and `MGET` are also supported, providing a way to set or get multiple keys in a single atomic operation.

Administrative utilities like `KEYS`, which supports glob-style pattern matching, and `FLUSHALL`, which clears all stored data, offer further control. GoRedis also implements client-side commands like `PING`, which checks if the server is alive (returning either `PONG` or an echoed message), and `HELLO`, which returns server details in RESP map format. The `CLIENT` command is included for future extensibility and currently responds with `OK`.

All commands conform strictly to RESP standards, ensuring compatibility with tools like `redis-cli` and RESP-aware clients. The command lifecycle—from parsing and validation to execution and response—is cleanly separated, ensuring both modularity and extensibility in the codebase. -->

| Type           | Format                             | Example                   |
| -------------- | ---------------------------------- | ------------------------- |
| Simple String  | `+OK\r\n`                          | Response to `PING`        |
| Error          | `-Error message\r\n`               | Invalid command           |
| Integer        | `:100\r\n`                         | Result of `INCR counter`  |
| Bulk String    | `$6\r\nfoobar\r\n`                 | Response to `GET key`     |
| Null           | `$-1\r\n`                          | `GET` on non-existent key |
| Array          | `*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n` | MGET result               |
| Map (Redis 6+) | `%2\r\n+key\r\n+value\r\n...`      | Used in `HELLO`           |

## Getting started

1. Clone the repository and have [Go installed](https://go.dev/doc/install)

```sh
# We recomend to fork and then use SSH to clone it.
git clone https://github.com/debarshee2004/goredis.git
```

2. Run the application

```sh
cd goredis
make
```

## Contributing

We welcome contributions to improve GoRedis!

### 🧱 To Contribute:

- Fork the repository
- Create a new branch (git checkout -b feature/your-feature)
- Commit your changes with clear messages
- Push to your fork and submit a PR

### 📌 Guidelines:

- Keep code readable and documented
- Match existing code patterns and formatting
- Write idiomatic Go
- Add comments for new commands or protocol changes

## Acknowledgement

Inspired by [Redis](https://redis.io)
