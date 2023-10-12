package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	masterServerAddress = "codmaster.activision.com:20510"
	getServersCmd       = "\xFF\xFF\xFF\xFFgetservers 1 full empty"
	getServersCmd1      = "ÿÿÿÿgetservers 1 full empty"
	getServersCmd2      = "ÿÿÿÿgetservers 2 full empty"
	getServersCmd3      = "ÿÿÿÿgetservers 3 full empty"
	getServersCmd4      = "ÿÿÿÿgetservers 4 full empty"
	getServersCmd5      = "ÿÿÿÿgetservers 6 full empty"
	specificServer      = "74.91.123.25:28960"
	udpTimeout          = 5 * time.Second
	bufferSize          = 65536
)

func main() {

	go startWebServer()

	/*for {
		fmt.Println("Veuillez sélectionner une option:")
		fmt.Println("1. Faire une requête sur le serveur spécifié:", specificServer)
		fmt.Println("2. Récupérer la liste de tous les serveurs COD 1")
		fmt.Println("3. Quitter")
		var choice string
		fmt.Scanln(&choice)
		switch choice {
		case "1":
			status, err := GetServerStatus(specificServer)
			if err != nil {
				fmt.Println("Erreur:", err)
			} else {
				for key, value := range status {
					fmt.Printf("%s: %s\n", key, value)
				}
			}
		case "2":
			queryAllServers()
		case "3":
			fmt.Println("Au revoir!")
			os.Exit(0)
		default:
			fmt.Println("Option invalide. Veuillez réessayer.")
		}
	}*/
}

func startWebServer() {
	http.HandleFunc("/api/servers", serveServersList)
	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.ListenAndServe(":8080", nil)
}

func GetServerStatus(serverAddress string) (map[string]string, error) {
	conn, err := net.DialTimeout("udp", serverAddress, udpTimeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	getStatusCmd := "\xFF\xFF\xFF\xFFgetstatus\n"
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

func queryAllServers() {
	conn, err := net.DialTimeout("udp", masterServerAddress, udpTimeout)
	if err != nil {
		fmt.Println("Erreur lors de la connexion au master server:", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte(getServersCmd))
	if err != nil {
		fmt.Println("Erreur lors de l'envoi de la commande getservers:", err)
		return
	}

	buffer := make([]byte, bufferSize)
	conn.SetReadDeadline(time.Now().Add(udpTimeout))
	length, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Erreur lors de la lecture de la réponse du master server:", err)
		return
	}

	var servers []string
	data := buffer[:length]
	startIndex := bytes.Index(data, []byte("\xFF\xFF\xFF\xFFgetserversResponse"))
	if startIndex == -1 {
		fmt.Println("Expected response prefix not found.")
		return
	}
	data = data[startIndex+24:]
	for len(data) > 0 {
		if data[0] == 0x5c && len(data) >= 7 {
			ip := fmt.Sprintf("%d.%d.%d.%d:%d", data[1], data[2], data[3], data[4], binary.BigEndian.Uint16(data[5:7]))
			servers = append(servers, ip)
			data = data[7:]
		} else {
			data = data[1:]
		}
	}

	results := make(chan string, len(servers))
	errs := make(chan error, len(servers))

	for _, server := range servers {
		go queryServer(server, results, errs)
	}

	for i := 0; i < len(servers); i++ {
		select {
		case result := <-results:
			fmt.Println(result)
		case err := <-errs:
			fmt.Println("Erreur: ", err)
		}
	}
}

func queryServer(server string, results chan<- string, errs chan<- error) {
	status, err := GetServerStatus(server)
	if err != nil {
		errs <- fmt.Errorf("Erreur lors de la récupération du statut pour le serveur %s : %v", server, err)
		return
	}

	var builder strings.Builder
	builder.WriteString("Informations pour le serveur: ")
	builder.WriteString(server)
	builder.WriteString("\n-------------------------------------\n")
	for key, value := range status {
		builder.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	builder.WriteString("-------------------------------------\n\n")
	results <- builder.String()
}

func queryAllServersForAPI(getServersCmd string) []map[string]string {
	serverAddresses := getServerAddresses(getServersCmd)
	var serverDetails []map[string]string
	for _, address := range serverAddresses {
		status, err := GetServerStatus(address)
		if err == nil {
			serverDetails = append(serverDetails, status)
		}
	}
	return serverDetails
}

func getServerAddresses(getServersCmd string) []string {
	conn, err := net.DialTimeout("udp", masterServerAddress, udpTimeout)
	if err != nil {
		fmt.Println("Erreur lors de la connexion au master server:", err)
		return nil
	}
	defer conn.Close()

	_, err = conn.Write([]byte(getServersCmd))
	if err != nil {
		fmt.Println("Erreur lors de l'envoi de la commande getservers:", err)
		return nil
	}

	buffer := make([]byte, bufferSize)
	conn.SetReadDeadline(time.Now().Add(udpTimeout))
	length, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Erreur lors de la lecture de la réponse du master server:", err)
		return nil
	}

	var serversList []string
	data := buffer[:length]
	startIndex := bytes.Index(data, []byte("\xFF\xFF\xFF\xFFgetserversResponse"))
	if startIndex == -1 {
		fmt.Println("Expected response prefix not found.")
		return nil
	}
	data = data[startIndex+24:]
	for len(data) > 0 {
		if data[0] == 0x5c && len(data) >= 7 {
			ip := fmt.Sprintf("%d.%d.%d.%d:%d", data[1], data[2], data[3], data[4], binary.BigEndian.Uint16(data[5:7]))
			serversList = append(serversList, ip)
			data = data[7:]
		} else {
			data = data[1:]
		}
	}
	return serversList
}

func serveServersList(w http.ResponseWriter, r *http.Request) {
	patch := r.URL.Query().Get("patch")

	var getServersCmd string
	switch patch {
	case "1":
		getServersCmd = getServersCmd1
	case "2":
		getServersCmd = getServersCmd2
	case "3":
		getServersCmd = getServersCmd3
	case "4":
		getServersCmd = getServersCmd4
	case "5":
		getServersCmd = getServersCmd5
	default:
		getServersCmd = getServersCmd1
	}

	servers := queryAllServersForAPI(getServersCmd)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}
