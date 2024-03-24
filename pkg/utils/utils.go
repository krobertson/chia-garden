package utils

import (
	"net"
	"strings"
)

// GetHostIP returns the net.IP of the default network interface on the machine.
func GetHostIP() net.IP {
	// Attempt a UDP connection to a dummy IP, which will cause the local end
	// of the connection to be the interface with the default route
	addr := &net.UDPAddr{IP: net.IP{1, 2, 3, 4}, Port: 1}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP
}

// ArrayFlags can be used with flags.Var to specify the a command line argument
// multiple timmes.
type ArrayFlags []string

// String returns a basic string concationation of all values.
func (i *ArrayFlags) String() string {
	return strings.Join(*i, ", ")
}

// Set is used to append a new value to the array by flags.Var.
func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}
