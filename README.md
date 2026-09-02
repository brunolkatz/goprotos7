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

## s7db

`s7db` is the current CLI for managing S7 DB schemas, packing/unpacking DB binaries, watching PLC values, heartbeat handling, and running `s7sim` scripts.

For more information, see: [./s7db/README.md](./s7db/README.md)

## Docs

- http://gmiru.com/article/s7comm/
- http://gmiru.com/article/s7comm-part2/
- https://github.com/Orange-Cyberdefense/awesome-industrial-protocols/blob/main/protocols/s7comm.md
- https://wiki.wireshark.org/samplecaptures#s7comm---s7-communication
