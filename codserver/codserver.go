package codserver

import (
	"bytes"
	"net"
	"strings"
	"time"
)

const (
	getStatusCmd = "\xFF\xFF\xFF\xFFgetstatus\n"
	udpTimeout   = 5 * time.Second
	bufferSize   = 65536
)

func GetServerStatus(serverAddress string) (map[string]string, error) {
	conn, err := net.DialTimeout("udp", serverAddress, udpTimeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(getStatusCmd))
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, bufferSize)
	conn.SetReadDeadline(time.Now().Add(udpTimeout))
	length, err := conn.Read(buffer)
	if err != nil {
		return nil, err
	}

	data := bytes.Split(buffer[:length], []byte("\n"))
	statusMap := make(map[string]string)

	parts := bytes.Split(data[1], []byte("\\"))
	for i := 1; i < len(parts)-1; i += 2 {
		key := string(parts[i])
		value := string(parts[i+1])
		statusMap[key] = value
	}

	var players []string
	for i := 2; i < len(data) && len(data[i]) > 0; i++ {
		players = append(players, string(data[i]))
	}
	statusMap["players_data"] = strings.Join(players, "\n")

	return statusMap, nil
}
