# goprotos7 - Open Industrial Protocol Server (S7-Compatible)

**goprotos7** is a standalone server that implements a protocol compatible with S7 communication, commonly used by industrial automation systems and PLCs. It is intended for testing, education, and development of SCADA/HMI tools and industrial simulators.

> ⚠️ This software is **not affiliated with Siemens AG** or any proprietary implementation.  
> It is an **independent, reverse-engineered protocol implementation** for compatibility and research purposes only.

---

## 🚀 Features

- [ ] Implements ISO-on-TCP (RFC 1006) with COTP session negotiation
  - [x] Supports request connection
  - [x] Supports response request connection
  - [x] Supports PDU (Protocol Data Unit) request
- [ ] Implements S7 communication protocol
  - [x] readvar request and response
  - [ ] writevar request and response
- [x] Simulates access to Data Blocks (DB)
  - Goals:
    - [X] ReadVar
    - [x] WriteVar
- [x] Simulate wrong request package response error

---

## Usage

Build:

```bash
go build -o goprotos7 ./cmd/goprotos7
```

Run:

| Env                    | Default Value | Description                                                           |
|------------------------|---------------|-----------------------------------------------------------------------|
| `--bin-folder` or `-b` |               | Target BINs files folder.                                             |
| `--port` or `-p`       | `102`         | The port to listen on. If empty, the default port `102` will be used. |

```bash
./goprotos7 --qlite-path ./db.sqlite --db-bin-path ./db
```

The service will start listening on port `102` by default, which is the standard port for S7 communication.

## DbTools

Used to create and maintain the database blocks used by goprotos7. Will create the bin file using the "db_variables" table from the SQLite database.

| Env                       | Default Value | Description                                                                            |
|---------------------------|---------------|----------------------------------------------------------------------------------------|
| `--qlite-path` or `-s`    | ``            | Store the default path for the SQLite database file. If empty, the `pwd` will be used. |
| `--db-bin-path` or `-b`   | ``            | Store the database BIN files path                                                      |
| `--flags.enable-webadmin` | `false`       | If `true` will enable the dbtools "frontend"                                           |
| `--log-level.sqlite`      | `SILENCE`     | Define the SQLite log level                                                            |

![dbtools_dashboard.png](./.docs/dbtools_dashboard.png)

## Docs

- http://gmiru.com/article/s7comm/
- http://gmiru.com/article/s7comm-part2/
- https://github.com/Orange-Cyberdefense/awesome-industrial-protocols/blob/main/protocols/s7comm.md
- https://wiki.wireshark.org/samplecaptures#s7comm---s7-communication
