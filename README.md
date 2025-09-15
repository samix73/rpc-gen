# rpc-gen

A simple Golang tool that generates RPC client code from Go interfaces in the current directory. It parses .go files, extracts interface definitions, and creates corresponding client code using Go's `net/rpc` package with gob encoding.

## Installation

Clone the repository and build the tool:

```bash
git clone https://github.com/samix73/rpc-gen.git
cd rpc-gen
go build -o rpc-gen main.go
```

Alternatively, install directly using Go:

```bash
go install github.com/samix73/rpc-gen@latest
```

## Usage

Run the tool in a directory containing your Go source files with interface definitions. It will generate client code for each interface found.

Command Line Options

- `-output <dir>`: Specify the output directory for generated files (default: current directory).
- `-verbose`: Enable verbose logging.

### Example

1. Create a Go file with an interface, e.g., `service.go`:

```go
package main

type MyService interface {
    DoSomething(request Request) (*Response, error)
}

type Request struct {
    Data string
}

type Response struct {
    Result string
}
```

2. Run the generator:

```bash
rpc-gen -output ./generated
```

This generates `myservice_client_gen.go` with the client code.

3. Use the generated client in your code:

```go
client, err := NewMyServiceClient("localhost:1234")
if err != nil {
    log.Fatal(err)
}
defer client.Close()

req := Request{Data: "hello"}
resp, err := client.DoSomething(req)
if err != nil {
    log.Fatal(err)
}
fmt.Println(resp.Result)
```

### Generated Code Features
- Registers request and response types with `gob`.
- Creates a client struct with methods matching the interface.
- Handles RPC calls over TCP with error wrapping.
- Includes a `Close()` method to close the connection.

