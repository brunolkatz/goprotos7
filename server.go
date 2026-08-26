package goprotos7

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

// TPKT Constants
// Uses the RFC 1006 TPKT header format
const (
	TPKTVersion  = 0x03
	TPKTReserved = 0x00
	DefaultPort  = 102
)

// COTP PDU Types
// Uses the ISO 8073-1 COTP header format
const (
	COTPConnectionRequest = 0xE0
	COTPConnectionConfirm = 0xD0
	COTPAcknowledgement   = 0x60
	COTPData              = 0xF0
	COTPDisconnectRequest = 0x80 // When a client sends the wrong request package
)

type Transport struct {
	Local bool
	Port  int
}

type Server struct {
	options *Options

	listener net.Listener
}

func New(opts ...ServerOption) (*Server, error) {
	ret := &Server{
		options: &Options{
			BinFilesFolder: "",
			Transport: &Transport{
				Local: false,       // Starts on 0.0.0.0
				Port:  DefaultPort, // The default port for S7 is 102
			},
		},
	}

	if opts != nil && len(opts) > 0 {
		for _, opt := range opts {
			if opt != nil {
				opt(ret.options)
			}
		}
	}
	return ret, nil
}

func (s *Server) Start() error {

	address := ""
	if s.options.Transport.Local {
		address = "127.0.0.1" // Not visible in the local network
	}
	if len(strings.Split(address, ":")) < 2 { // Add the port
		address = address + ":" + fmt.Sprintf("%d", s.options.Transport.Port)
	}

	adr, err := net.ResolveTCPAddr("tcp4", address)
	if err != nil {
		return errors.New(fmt.Sprintf("Error resolving address %s: %+v", address, err))
	}

	listener, err := net.ListenTCP("tcp4", adr)
	if err != nil {
		return errors.New(fmt.Sprintf("Error binding to port 102 (try sudo?): %+v", err))
	}
	defer listener.Close()
	s.listener = listener
	log.Println(fmt.Sprintf("[S7_SERVER] Listening on %s", address))

	// Start listening upcoming requests connections
	cid := uint32(0) // Connection id (just a counter)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		connHandler := NewConnection(cid, conn, s.options)
		go connHandler.Start()
		log.Printf("[S7_SERVER] New client connected: %s (CID: %d)", conn.RemoteAddr(), cid)
		cid++ // Increment connection ID for the next client
	}
}

type Connection struct {
	CId     uint32
	options *Options
	conn    *net.TCPConn

	// The closed state of the current connection
	isClosed bool
}

func NewConnection(cid uint32, conn *net.TCPConn, options *Options) *Connection {
	return &Connection{
		CId:      cid,
		conn:     conn,
		isClosed: false,
		options:  options,
	}
}

func (c *Connection) Stop() {
	// If the current link is closed
	if c.isClosed == true {
		return
	}
	c.isClosed = true

	// Close the socket link
	_ = c.conn.Close()
}

// Start the connection and let the current connection start to work
func (c *Connection) Start() {
	go c.StartReader()
}

func (c *Connection) DoMsgHandler(msg *Message) {
	if msg.COTPHeader.PDUType == COTPData {
		switch msg.S7Request.FunctionCode {
		case S7FuncReadVar:
			c.eventS7FuncReadVar(msg, c.conn)
		default:
			return
		}
	}
}

// StartReader handles the incoming connection
func (c *Connection) StartReader() {
	defer c.conn.Close()

	log.Printf("[SERVER] New client: %s", c.conn.RemoteAddr())

	// Step 1: read Connection Request
	buffer, err := readConn(c.conn)
	if err != nil {
		log.Println("Read error:", err)
		return
	}
	msg, err := unpack(buffer)
	if err != nil {
		log.Println("Unpack error:", err)
		return
	}

	// Check if the message is a Connection Request
	if msg.COTPHeader.PDUType != COTPConnectionRequest {
		// Write a Disconnect request
		ret := &Message{
			TPKTHeader: msg.TPKTHeader,
			COTPHeader: msg.COTPHeader,
		}
		pack, _ := ret.Pack(COTPDisconnectRequest)
		_, err = c.conn.Write(pack)
		if err != nil {
			log.Println("Write error:", err)
			return
		}
		// Close the connection
		_ = c.conn.Close()
		return
	} else { // Accept the connection and let the client know
		ret := &Message{
			TPKTHeader: msg.TPKTHeader,
			COTPHeader: msg.COTPHeader,
		}
		pack, _ := ret.Pack(COTPConnectionConfirm)
		_, err = c.conn.Write(pack)
		if err != nil {
			log.Println("Write error:", err)
			return
		}
		log.Println("[SERVER] Connection Confirm sent")

		// Step 2: send Connection Confirm
		buffer, err = readConn(c.conn)
		if err != nil {
			log.Println("Read error:", err)
			return
		}
		msg, err = unpack(buffer)
		if err != nil {
			log.Println("Unpack error:", err)
			return
		}
		if msg.S7Request.FunctionCode != S7FuncSetupCommunication {
			log.Println("Not a Setup Communication request")
			return
		}
		var res *Message
		res, err = getS7ParamSetupCommunicationResponse(msg)
		if err != nil {
			log.Println("Error getting Setup Communication response:", err)
			return
		}
		pack, err := res.Pack(COTPData)
		if err != nil {
			log.Println("Error packing response:", err)
			return
		}
		_, err = c.conn.Write(pack)
		log.Println("[SERVER] Setup Communication response sent successfully")

		// Everything is ok, now we can read/write data
		for {
			buffer, err = readConn(c.conn)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					log.Println("[SERVER] Connection timed out")
					continue // idle client, wait for next message
				}
				log.Println("Read error:", err)
				return // disconnect client on real error
			}

			msg, err = unpack(buffer)
			if err != nil {
				if errors.Is(err, ErrorFunctionCodeNotSupported) {
					ret := getS7FunctionCodeNotSupportedResponse(msg)
					pack, _ := ret.Pack(COTPData)
					_, err = c.conn.Write(pack)
					continue
				}
				// TODO: On error send back to the client some error message
				log.Println("[CON_HANDLER] Unpack error:", err)
				continue
			}
			if msg.S7Request == nil {
				continue
			}
			go c.DoMsgHandler(msg)
		}
	}
}

//   ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
//   ┃                                                    helpers                                                    ┃
//   ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

func readConn(c net.Conn) ([]byte, error) {

	// Step 1: Read 4 bytes (TPKT Header)
	header := make([]byte, 4)
	if _, err := c.Read(header); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("connection closed by client - 1")
		}
		return nil, fmt.Errorf("error reading TPKT header: %w", err)
	}

	// Step 2: Read TPKT Length (bytes 2 and 3 are the packet length)
	packetLength := int(binary.BigEndian.Uint16(header[2:4]))
	if packetLength <= 0 {
		return nil, fmt.Errorf("invalid TPKT packet length: %d", packetLength)
	}

	// Step 3: Read the remaining packet (packetLength - 4 because header already read)
	body := make([]byte, packetLength-4)
	if _, err := c.Read(body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("connection closed by client - 2")
		}
		return nil, fmt.Errorf("error reading packet body: %w", err)
	}

	// Step 4: Return a full packet (header and body)
	return append(header, body...), nil
}
