# goprotos7 - Open Industrial Protocol Server (S7-Compatible)

**goprotos7** is a standalone server that implements a protocol compatible with S7 communication, commonly used by industrial automation systems and PLCs. It is intended for testing, education, and development of SCADA/HMI tools and industrial simulators.

> ⚠️ This software is **not affiliated with Siemens AG** or any proprietary implementation.  
> It is an **independent, reverse-engineered protocol implementation** for compatibility and research purposes only.

---

## 🚀 Features

- Implements ISO-on-TCP (RFC 1006) with COTP session negotiation
- Supports S7-compatible read requests
- Simulates access to Data Blocks (DB)
     - Goals: Inputs (I), Outputs (Q), and Memory (M) 
- Built using Go for performance and portability
- CLI-based server configuration

---
