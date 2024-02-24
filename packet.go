package main

import (
	"bufio"
	"crypto/aes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

type Packet struct {
	Header string
	Data   string
}

func parsePacket(reader *bufio.Reader) (*Packet, error) {
	lengthBuf := make([]byte, 4)
	_, err := reader.Read(lengthBuf)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)

	messageBuf := make([]byte, length)
	_, err = reader.Read(messageBuf)
	if err != nil {
		return nil, err
	}

	message := string(messageBuf)
	fmt.Printf("Message: %s\n", message)

	var packet struct {
		Header string          `json:"header"`
		Data   json.RawMessage `json:"data"`
	}
	err = json.Unmarshal([]byte(message), &packet)
	if err != nil {
		return nil, err
	}

	data := string(packet.Data)
	if len(data) > 0 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}

	return &Packet{Header: packet.Header, Data: data}, nil
}

func parseEncryptedPacket(reader *bufio.Reader, encryptionKey string) (*Packet, error) {
	lengthBuf := make([]byte, 4)
	_, err := reader.Read(lengthBuf)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)

	fmt.Println("length", length)

	messageBuf := make([]byte, length)
	_, err = reader.Read(messageBuf)
	if err != nil {
		return nil, err
	}

	encoded := string(messageBuf)
	fmt.Printf("Message: %s\n", encoded)

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	key, err := generateKey(encryptionKey)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	decrypter := NewECBDecrypter(block)
	decryptedText := make([]byte, len(decoded))
	decrypter.CryptBlocks(decryptedText, decoded)

	finalText := string(pKCS5Unpadding(decryptedText))
	fmt.Println("finalText", finalText)

	var packet struct {
		Header string          `json:"header"`
		Data   json.RawMessage `json:"data"`
	}
	err = json.Unmarshal([]byte(finalText), &packet)
	if err != nil {
		return nil, err
	}

	data := string(packet.Data)
	if len(data) > 0 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}

	return &Packet{Header: packet.Header, Data: data}, nil
}

func sendEncryptedPacket(conn net.Conn, header string, data string, encryptionKey string) {
	key, err := generateKey(encryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	plaintext := []byte(`{"header":"` + header + `","data":"` + data + `"}`)

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal(err)
	}
	paddedText := pKCS5Padding(plaintext, block.BlockSize())

	encrypter := NewECBEncrypter(block)
	ciphertext := make([]byte, len(paddedText))
	encrypter.CryptBlocks(ciphertext, paddedText)

	ciphertextEncoded := base64.StdEncoding.EncodeToString(ciphertext)

	length := uint32(len(ciphertextEncoded))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)

	_, err = conn.Write(append(lengthBytes, []byte(ciphertextEncoded)...))
	if err != nil {
		log.Fatal(err)
	}
}

func sendPacket(conn net.Conn, header string, data string) {
	packet := `{"header":"` + header + `","data":"` + data + `"}`

	length := uint32(len(packet))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)

	_, err := conn.Write(append(lengthBytes, []byte(packet)...))
	if err != nil {
		log.Fatal(err)
	}
}
