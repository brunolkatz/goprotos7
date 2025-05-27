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
- [ ] Simulates access to Data Blocks (DB)
  - Goals:
    - [X] ReadVar
    - [ ] WriteVar
- [ ] Simulate wrong request package response error

---

## Webadmin

Can be used to add and change the values of the simulated server. You can also fill the `db_variables` table with your variables
and set the `--flags.disable-webadmin` to just create the binary file with the variables if you don't want to change the variables in runtime 

## Docs

- http://gmiru.com/article/s7comm/
- http://gmiru.com/article/s7comm-part2/
- https://github.com/Orange-Cyberdefense/awesome-industrial-protocols/blob/main/protocols/s7comm.md
- https://wiki.wireshark.org/samplecaptures#s7comm---s7-communication
