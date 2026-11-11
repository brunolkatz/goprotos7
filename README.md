# goprotos7 - Open Industrial Protocol Server (S7-Compatible)

**goprotos7** is a standalone server that implements a protocol compatible with S7 communication, commonly used by industrial automation systems and PLCs. It is intended for testing, education, and development of SCADA/HMI tools and industrial simulators.

> ⚠️ This software is **not affiliated with Siemens AG** or any proprietary implementation.  
> It is an **independent, reverse-engineered protocol implementation** for compatibility and research purposes only.

---

## 🚀 Features

- [x] Implements ISO-on-TCP (RFC 1006) with COTP session negotiation
  - [x] Supports request connection
  - [x] Supports response request connection
  - [x] Supports PDU (Protocol Data Unit) request
- [x] Implements S7 communication protocol
  - [x] readvar request and response
  - [x] writevar request and response
  - [x] user-data SZL request for CPU info (`GetCPUInfo`)
- [x] Simulates access to Data Blocks (DB)
  - Goals:
    - [X] ReadVar
    - [x] WriteVar
- [x] Simulate wrong request package response error

---

## Installation

Install latest binaries with Go:

```bash
# Server
go install github.com/brunolkatz/goprotos7/cmd/goprotos7@latest

# s7db CLI
go install github.com/brunolkatz/goprotos7/s7db/cmd/s7db@latest
```

Build locally:

```bash
go build -o goprotos7 ./cmd/goprotos7
go build -o s7db ./s7db/cmd/s7db
```

Run:

| Env                    | Default Value | Description                                                           |
|------------------------|---------------|-----------------------------------------------------------------------|
| `--bin-folder` or `-b` |               | Target BINs files folder.                                             |
| `--port` or `-p`       | `102`         | The port to listen on. If empty, the default port `102` will be used. |

```bash
goprotos7 --bin-folder ./db
```

The service will start listening on port `102` by default, which is the standard port for S7 communication.

## Writing a WSTRING to a simulated DB

`Connection.AGWriteDBWString` writes an S7-1200 WSTRING to a local `DB<n>.bin`
file. Supply the field's byte offset and declared capacity in 16-bit characters:

```go
err := conn.AGWriteDBWString(1, 10, 20, "Hello")
```

The file must already contain the complete field. Its layout is a two-byte
big-endian maximum length, a two-byte big-endian current length, and two-byte
big-endian characters. The method writes the full `4 + 2*capacity` bytes and
zero-fills unused character slots. Capacities must be 1 through 16382; invalid
UTF-8, characters outside the Unicode Basic Multilingual Plane, and values
exceeding the capacity return errors.

The S7 WriteVar server also accepts an already encoded WSTRING as ordinary DB
bytes. These APIs write to the simulated DB, not directly to a PLC.

## s7db

`s7db` is the current CLI for managing S7 DB schemas, packing/unpacking DB binaries, watching PLC values, heartbeat handling, and running `s7sim` scripts.

For more information, see: [./s7db/README.md](./s7db/README.md)

## Docs

- http://gmiru.com/article/s7comm/
- http://gmiru.com/article/s7comm-part2/
- https://github.com/Orange-Cyberdefense/awesome-industrial-protocols/blob/main/protocols/s7comm.md
- https://wiki.wireshark.org/samplecaptures#s7comm---s7-communication
