package fit

import (
	"fmt"
	"net"
	"strings"
)

func GetMacAddrs() (macAddrs []string) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		return macAddrs
	}

	for _, netInterface := range netInterfaces {
		macAddr := netInterface.HardwareAddr.String()
		if len(macAddr) == 0 {
			continue
		}

		macAddrs = append(macAddrs, macAddr)
	}
	return macAddrs
}

func GetOutBoundIP() (ip string, err error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return "", err
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip = strings.Split(localAddr.String(), ":")[0]
	return ip, nil
}

// GetRandomAvPort Randomly obtain an available port number.
func GetRandomAvPort() (int, error) {
	listen, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listen.Close()
	return listen.Addr().(*net.TCPAddr).Port, nil
}

func GetListenPort(ls net.Listener) int {
	return ls.Addr().(*net.TCPAddr).Port
}

// GetRandomAvPortAndHost Obtain the IP+random available port number of this machine.
func GetRandomAvPortAndHost() (string, error) {
	localIp, err := GetOutBoundIP()
	if err != nil {
		return "", err
	}
	port, err := GetRandomAvPort()
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(localIp, fmt.Sprintf("%d", port)), nil
}
