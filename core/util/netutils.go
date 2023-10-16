package util

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/rate"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//Class        Starting IPAddress    Ending IP Address    # of Hosts
//A            10.0.0.0              10.255.255.255       16,777,216
//B            172.16.0.0            172.31.255.255       1,048,576
//C            192.168.0.0           192.168.255.255      65,536
//Link-local-u 169.254.0.0           169.254.255.255      65,536
//Link-local-m 224.0.0.0             224.0.0.255          256
//Local        127.0.0.0             127.255.255.255      16777216

// TestPort check port availability
func TestPort(port int) bool {
	host := ":" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", host)
	if ln != nil {
		err := ln.Close()
		if err != nil {
			panic(err)
		}
	}

	if err != nil {
		log.Debugf("can't listen on port %s, %s", host, err)
		return false
	}
	return true
}

func WaitServerUp(addr string, duration time.Duration) error {
	start := time.Now()
	d := net.Dialer{Timeout: duration}
check:
	conn, err := d.Dial("tcp", addr)
	if conn != nil {
		conn.Close()
	}
	if err != nil {
		log.Trace("still not there, ",addr)
		goto wait
	}
	return nil

wait:
	if time.Now().Sub(start) > duration {
		log.Trace("retry enough, forget about it")
		return errors.New("timeout")
	}

	time.Sleep(100 * time.Millisecond)
	goto check

	return nil
}

// TestListenOnTCPPort check availability of port with ip
func TestListenOnTCPPort(ip string, port int) bool {

	log.Tracef("testing port %s:%d", ip, port)
	host := ip + ":" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", host)
	if ln != nil {
		err := ln.Close()
		if err != nil {
			panic(err)
		}
	}

	if err != nil {
		log.Debugf("can't listen on port %s, %s", host, err)
		return false
	}
	return true
}

// GetAvailablePort get valid port to listen, if the specify port is not available, auto choose the next one
func GetAvailablePort(ip string, port int) int {

	maxRetry := 500

	for i := 0; i < maxRetry; i++ {
		ok := TestListenOnTCPPort(ip, port)
		if ok {
			log.Trace("get available port: ", port)
			return port
		}
		port++
	}

	panic(errors.New("no ports available"))
}

func TestTCPPort(host string,port int,duration time.Duration)bool  {
	return TestTCPAddress(fmt.Sprintf("%v:%v",host,port),duration)
}

func TestTCPAddress(host string,timeout time.Duration)bool  {
	if !rate.GetRateLimiterPerSecond("test_tcp_address",host,1).Allow(){
		time.Sleep(1*time.Second)
	}
	log.Debug("testing ip:",host,",timeout:",timeout)
	conn, err := net.DialTimeout("tcp", host, timeout)
	if conn!=nil{
		conn.Close()
		if err==nil{
			return true
		}
	}
	if err!=nil{
		log.Debug(err,",",conn!=nil,",",timeout)
	}
	return false
}

// AutoGetAddress get valid address to listen, if the specify port is not available, auto choose the next one
func AutoGetAddress(addr string) string {

	//skip for ipv6 family
	if strings.Contains(addr,"::"){
		return addr
	}

	if strings.Index(addr, ":") < 0 {
		panic(errors.New("invalid address, eg ip:port, " + addr))
	}

	array := strings.Split(addr, ":")
	p, _ := strconv.Atoi(array[1])
	port := GetAvailablePort(GetSafetyInternalAddress(array[0]), p)
	array[1] = strconv.Itoa(port)
	return strings.Join(array, ":")
}

func GetSafetyInternalAddress(addr string) string {
	//skip for ipv6 family
	if strings.Contains(addr,"::"){
		return addr
	}

	if strings.Contains(addr, ":") {
		array := strings.Split(addr, ":")
		if array[0] == "0.0.0.0" {
			array[0], _ = GetIntranetIP()
		}
		return strings.Join(array, ":")
	}

	return addr
}

// GetValidAddress get valid address, input: :8001 -> output: 127.0.0.1:8001
func GetValidAddress(addr string) string {
	//skip for ipv6 family
	if strings.Contains(addr,"::"){
		return addr
	}

	if strings.Index(addr, ":") >= 0 {
		array := strings.Split(addr, ":")
		if len(array[0]) == 0 {
			array[0] = "127.0.0.1"
			addr = strings.Join(array, ":")
		}
	}
	return addr
}

func GetAddress(adr string) *net.TCPAddr {
	addr, err := net.ResolveTCPAddr("tcp", adr)
	if err != nil {
		panic(err)
	}
	return addr
}

func IsPublicIP(IP net.IP) bool {
	if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := IP.To4(); ip4 != nil {
		switch true {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		default:
			return true
		}
	}
	return false
}

func GetIntranetIP() (string, error) {
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		return "", err
	}

	for _, address := range addrs {

		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", errors.New("can't get intranet ip")
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIPs() []string {
	addrs, err := net.InterfaceAddrs()
	ips := []string{}
	if err != nil {
		return ips
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips
}

func CheckIPBinding(ip string) (bool, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				if ip == ipnet.IP.String() {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func Ipv4MaskString(m []byte) string {
	if len(m) != 4 {
		panic("ipv4Mask: len must be 4 bytes")
	}

	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}

func GetPublishNetworkDeviceInfo(pattern string)(dev string,ip string,mask string,err error)  {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {

					//fmt.Println(i.Name)
					//fmt.Println(ipnet.IP.String())
					//fmt.Println(Ipv4MaskString(ipnet.Mask))
					//fmt.Println(i.HardwareAddr.String())
					//fmt.Println(i.MTU)

					if pattern!=""{
						ip:=ipnet.IP.String()
						if !RegexPatternMatch(pattern,ip){
							continue
						}
					}
					return i.Name,ipnet.IP.String(),Ipv4MaskString(ipnet.Mask),nil
				}
			}
		}
	}
	return "","","",errors.New("no publishable network device found")
}

func GetIPPrefix(ip string)string  {
	index:=strings.LastIndex(ip,".")
	return SubString(ip,0,index)
}


func ClientIP(r *http.Request) string {
	ip := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if ip != "" {
		return ip
	}

	ip = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	if ip != "" {
		return ip
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		if ip == "::1" {
			ip = "127.0.0.1"
		}
		return ip
	}

	return ""
}

func UnifyLocalAddress(host string)string  {
	//unify host
	if ContainStr(host, "localhost") {
		host = strings.Replace(host, "localhost", "127.0.0.1", -1)
	} else if ContainStr(host, "[::1]") {
		host = strings.Replace(host, "[::1]", "127.0.0.1", -1)
	} else if ContainStr(host, "::1") {
		host = strings.Replace(host, "::1", "127.0.0.1", -1)
	}
	return host
}