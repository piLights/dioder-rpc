package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"gitlab.com/piLights/dioder-rpc/configuration"
	"gitlab.com/piLights/dioder-rpc/logging"

	"github.com/davecheney/mdns"
)

const hexDigit = "0123456789abcdef"

// reverseAddr returns the in-addr.arpa. or ip6.arpa. hostname of the IP
// address suitable for reverse DNS (PTR) record lookups or an error if it fails
// to parse the IP address. - this is from the oficial golang-code
func reverseAddr(addr string) (string, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", errors.New("unrecognized address: " + addr)
	}
	if ip.To4() != nil {
		return strconv.Itoa(int(ip[15])) + "." + strconv.Itoa(int(ip[14])) + "." + strconv.Itoa(int(ip[13])) + "." +
			strconv.Itoa(int(ip[12])) + ".in-addr.arpa.", nil
	}
	// Must be IPv6
	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	// Add it, in reverse, to the buffer
	for i := len(ip) - 1; i >= 0; i-- {
		v := ip[i]
		buf = append(buf, hexDigit[v&0xF])
		buf = append(buf, '.')
		buf = append(buf, hexDigit[v>>4])
		buf = append(buf, '.')
	}
	// Append "ip6.arpa." and return (buf already has the final .)
	buf = append(buf, "ip6.arpa."...)
	return string(buf), nil
}

//removeWhitespaces removes all whitespaces from the given string
func removeWhitespaces(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}

//startServer starts the GRPC-server and binds to the defined address
func startAutoConfigurationServer() {

	_, port, error := net.SplitHostPort(configuration.DioderConfiguration.BindTo)
	if error != nil {
		log.Fatal(error)
	}

	if configuration.DioderConfiguration.UseAvahi {
		// os Exec avahi-publish-service -s dioderServer _dioder._tcp 13337
		_, err := exec.Command("avahi-publish-service", "-s", configuration.DioderConfiguration.ServerName, "_dioder._tcp", port).Output()
		if err != nil {
			logging.FatalChan <- err
		}
	} else {

		if configuration.DioderConfiguration.Debug {
			logging.LogChan <- fmt.Sprintf("Binding to %s", configuration.DioderConfiguration.BindTo)
		}

		//Publish the ServerName
		publishRecord(`_dioder._tcp.local. 10 IN TXT "` + configuration.DioderConfiguration.ServerName + `"`)

		//Register _dioder._tcp on the local mDNS domain
		publishRecord("_services._dns-sd._udp.local. 10 IN PTR _dioder._tcp.local.")

		cleanHostName := removeWhitespaces(configuration.DioderConfiguration.ServerName)
		//A record for servername.local for every IPv4 address
		//AAAA record for serverName.local for every IPv6 address
		publishARecords(cleanHostName)

		// SRV -> _dioder._tcp.local 10 IN SRV 0 0 PORT HOST
		createSRVRecord(cleanHostName, port)
	}
}

//publishARecords publishes an A or AAAA record on the given hostname with every interface-address
func publishARecords(hostName string) {
	addressList, error := net.InterfaceAddrs()
	if error != nil {
		log.Fatal(error)
	}

	for _, address := range addressList {
		ipnet, ok := address.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.String() != "" {
				// Do not publish IPv4 records if IPv4 is disabled
				if configuration.DioderConfiguration.IPv4Only && ipnet.IP.To4() == nil {
					continue
				}

				// Do not publish IPv6 records if IPv6 is disabled
				if configuration.DioderConfiguration.IPv6Only && ipnet.IP.To4() != nil {
					continue
				}

				ipAddress, _, error := net.ParseCIDR(address.String())
				if error != nil {
					log.Fatal(error)
				}

				if ipnet.IP.To4() != nil {
					publishRecord(hostName + ".local. 10 IN A " + ipAddress.String())
				} else {
					publishRecord(hostName + ".local. 10 IN AAAA " + ipAddress.String())
				}

				arpaAddr, error := reverseAddr(ipAddress.String())
				if error != nil {
					log.Fatal(error)
				}

				publishRecord(arpaAddr + " 10 IN PTR _dioder._tcp.local.")
			}
		}
	}
}

//createSRVRecord creates an SRV record announcing the service on the given host:port
func createSRVRecord(hostName, port string) {
	var srvRecord = "_dioder._tcp.local. 10 IN SRV 0 0 " + port + " " + hostName + ".local."
	publishRecord(srvRecord)
}

//publishRecord publishes an record
func publishRecord(resourceRecord string) {
	if configuration.DioderConfiguration.Debug {
		logging.LogChan <- fmt.Sprintf("Setting resourceRecord: %s", resourceRecord)
	}

	error := mdns.Publish(resourceRecord)
	if error != nil {
		log.Fatalf(`Unable to publish record "%s": %v`, resourceRecord, error)
	}
}
