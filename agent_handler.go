package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strconv"
	"sync"
)

type handlerMap map[string]func(net.Conn, string) error

var messageHandlers handlerMap

func init() {
	messageHandlers = handlerMap{
		"initHandshake":   initHandshake,
		"listRunningBots": agentData,
		"wrapperData":     wrapperData,
		"updateBot":       updateBot,
	}
}

func handleAgentConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)

	defer func() {
		Agents.Lock()
		defer Agents.Unlock()
		for uuid, agent := range Agents.m {
			if agent.Conn == conn {
				err := agent.Conn.Close()
				if err != nil {
					return
				}
				onlineAccounts.WithLabelValues(strconv.Itoa(agent.CustomerId), uuid).Set(0)
				onlineAgents.WithLabelValues(uuid, strconv.Itoa(agent.CustomerId)).Dec()
				deleteAgent(uuid)
				break
			}
		}
	}()

	sendPacket(conn, "initHandshake", "0.1")

	for {
		var packet *Packet
		var err error

		agent := getAgentByConn(conn)

		if agent.UUID != "" && agent.EncryptionKey != "" {
			log.Println("Reading encrypted packet")
			packet, err = parseEncryptedPacket(reader, agent.EncryptionKey)
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			log.Println("Reading packet")
			packet, err = parsePacket(reader)
			if err != nil {
				log.Println(err)
				return
			}
		}

		log.Println("Packet read")

		if err != nil {
			log.Println(err)
			return
		}

		header, data := packet.Header, packet.Data

		if _, ok := messageHandlers[header]; !ok {
			log.Printf("Unknown packet: Header: \"%s\", Data: \"%s\"\n", header, data)
		} else {
			log.Printf("Received packet: Header: \"%s\", Data: \"%s\"\n", header, data)
		}

		err = messageHandlers[header](conn, data)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

type Handshake struct {
	AgentUuid string `json:"machineId"`
}

func initHandshake(conn net.Conn, data string) error {
	var handshake Handshake
	err := json.Unmarshal([]byte(data), &handshake)
	if err != nil {
		return err
	}

	if handshake.AgentUuid == "" {
		return errors.New("connection refused: Agent UUID is empty")
	}

	var customerId int
	customerId, err = strconv.Atoi(sendWebRequest("customerId", "{\"uuid\":\""+handshake.AgentUuid+"\"}"))

	if err != nil {
		return err
	}

	log.Println("Customer ID: " + strconv.Itoa(customerId))

	if customerId <= 0 {
		return errors.New("connection refused: Owner not found for agent")
	}

	var allowedAgents int
	allowedAgents, err = strconv.Atoi(sendWebRequest("allowedClients", "{\"customerId\":"+strconv.Itoa(customerId)+"}"))

	if err != nil {
		return err
	}

	log.Println("Allowed agents: " + strconv.Itoa(allowedAgents))

	if allowedAgents <= 0 || len(getAgentsByCustomerId(customerId)) >= allowedAgents {
		return errors.New("connection refused: Max agents reached for customer " + strconv.Itoa(customerId))
	}

	if isAgentConnected(handshake.AgentUuid) {
		return errors.New("connection refused: Agent already connected")
	}

	var encryptionKey string
	encryptionKey = sendWebRequest("agentKey", "{\"uuid\":\""+handshake.AgentUuid+"\"}")
	log.Println("Encryption key: " + encryptionKey)

	if encryptionKey == "" {
		return errors.New("connection refused: Encryption key not found")
	}

	log.Println("Agent connected: UUID: " + handshake.AgentUuid + ", CustomerId: " + strconv.Itoa(customerId))

	onlineAccounts.WithLabelValues(strconv.Itoa(customerId), handshake.AgentUuid).Set(0)
	onlineAgents.WithLabelValues(handshake.AgentUuid, strconv.Itoa(customerId)).Inc()
	addAgent(handshake.AgentUuid, Agent{Conn: conn, CustomerId: customerId, UUID: handshake.AgentUuid, EncryptionKey: encryptionKey, RunningBots: []Bot{}})
	sendPacket(conn, "handshakeOk", strconv.Itoa(customerId))
	return nil
}

type RunningBotsData map[string][]map[string]string

func agentData(conn net.Conn, data string) error {
	agent := getAgentByConn(conn)
	if agent.CustomerId <= 0 {
		return errors.New("agent not found in database")
	}

	var runningBotsData RunningBotsData
	err := json.Unmarshal([]byte(data), &runningBotsData)
	if err != nil {
		return err
	}

	for agentId, statuses := range runningBotsData {
		serverOwner := getCustomerIdByAgentUuid(agentId)
		onlineAccounts.WithLabelValues(strconv.Itoa(serverOwner), agentId).Set(float64(len(statuses)))
	}

	sendWebRequest("agentData", data)
	return nil
}

func wrapperData(conn net.Conn, data string) error {
	agent := getAgentByConn(conn)
	if agent.CustomerId <= 0 {
		return errors.New("agent not found in database")
	}

	sendWebRequest("wrapperData", data)

	return nil
}

type UpdateBotData struct {
	Id     int    `json:"Id"`
	Status string `json:"Status"`
}

func updateBot(conn net.Conn, data string) error {
	agent := getAgentByConn(conn)
	if agent.CustomerId <= 0 {
		return errors.New("agent not found in database")
	}

	var updateData UpdateBotData
	err := json.Unmarshal([]byte(data), &updateData)
	if err != nil {
		return err
	}

	err = updateBotInAgent(agent.UUID, updateData.Id, updateData.Status)
	if err != nil {
		return err
	}

	switch updateData.Status {
	case "Completed":
		statuses.WithLabelValues(strconv.Itoa(agent.CustomerId), agent.UUID, "Completed").Inc()
	case "Banned":
		statuses.WithLabelValues(strconv.Itoa(agent.CustomerId), agent.UUID, "Banned").Inc()
	case "Locked":
		statuses.WithLabelValues(strconv.Itoa(agent.CustomerId), agent.UUID, "Locked").Inc()
	}

	sendWebRequest("updateBot", "{\"Id\":"+strconv.Itoa(updateData.Id)+",\"Status\":\""+updateData.Status+"\"}")

	return nil
}

func updateBotInAgent(uuid string, internalId int, newStatus string) error {
	Agents.Lock()
	defer Agents.Unlock()

	agent, ok := Agents.m[uuid]
	if !ok {
		return errors.New("agent not found")
	}

	botFound := false
	for i, bot := range agent.RunningBots {
		if bot.InternalId == internalId {
			if newStatus == "Stopped" {
				agent.RunningBots = removeBotFromAgent(agent.RunningBots, internalId)
			} else {
				agent.RunningBots[i].Status = newStatus
			}
			botFound = true
			break
		}
	}

	if !botFound && newStatus != "Stopped" {
		agent.RunningBots = append(agent.RunningBots, Bot{InternalId: internalId, Status: newStatus})
	}

	Agents.m[uuid] = agent

	return nil
}

func removeBotFromAgent(bots []Bot, id int) []Bot {
	for i, bot := range bots {
		if bot.InternalId == id {
			return append(bots[:i], bots[i+1:]...)
		}
	}
	return bots
}

type Bot struct {
	InternalId   int
	ScriptName   string
	ScriptParams string
	Status       string
}

type Agent struct {
	Conn          net.Conn
	CustomerId    int
	UUID          string
	EncryptionKey string
	RunningBots   []Bot
}

var Agents = struct {
	sync.RWMutex
	m map[string]Agent
}{m: make(map[string]Agent)}

func addAgent(uuid string, agent Agent) {
	Agents.Lock()
	defer Agents.Unlock()
	Agents.m[uuid] = agent
}

func getAgent(uuid string) Agent {
	Agents.RLock()
	defer Agents.RUnlock()
	agent := Agents.m[uuid]
	return agent
}

func deleteAgent(uuid string) {
	Agents.Lock()
	defer Agents.Unlock()
	delete(Agents.m, uuid)
}

func isAgentConnected(uuid string) bool {
	Agents.RLock()
	defer Agents.RUnlock()
	_, ok := Agents.m[uuid]
	return ok
}

func getAgentsByCustomerId(customerId int) []Agent {
	Agents.RLock()
	defer Agents.RUnlock()

	var agents []Agent
	for _, agent := range Agents.m {
		if agent.CustomerId == customerId {
			agents = append(agents, agent)
		}
	}

	return agents
}

func getAgentByConn(conn net.Conn) Agent {
	Agents.RLock()
	defer Agents.RUnlock()
	for _, agent := range Agents.m {
		if agent.Conn == conn {
			return agent
		}
	}
	return Agent{}
}

func getCustomerIdByAgentUuid(agentUuid string) int {
	Agents.RLock()
	defer Agents.RUnlock()
	for id, agent := range Agents.m {
		if id == agentUuid {
			return agent.CustomerId
		}
	}
	return 0
}
