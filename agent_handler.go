package main

import (
	"bufio"
	"log"
	"net"
)

type handlerMap map[string]func(net.Conn, string) error

var messageHandlers handlerMap

func init() {
	messageHandlers = handlerMap{
		"initHandshake": initHandshake,
		"agentData":     agentData,
		"wrapperData":   wrapperData,
		"updateBot":     updateBot,
	}
}

func handleAgentConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		var packet *Packet
		var err error

		//TODO if handshake completed for current conn, encrypt packets
		packet, err = parsePacket(reader)

		if err != nil {
			log.Println(err)
			return
		}

		header, data := packet.Header, packet.Data

		if _, ok := messageHandlers[header]; !ok {
			log.Printf("Unknown packet: Header: \"%s\", Data: \"%s\"\n", header, data)
			return
		}

		err = messageHandlers[header](conn, data)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func initHandshake(conn net.Conn, data string) error {
	return nil
}

func agentData(conn net.Conn, data string) error {
	return nil
}

func wrapperData(conn net.Conn, data string) error {
	return nil
}

func updateBot(conn net.Conn, data string) error {
	return nil
}
