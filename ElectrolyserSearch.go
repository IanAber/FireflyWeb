package main

import (
	"fmt"
	"github.com/simonvetter/modbus"
	"log"
	"net"
	"time"
)

/*
GetOurIP will returns the preferred ip address of the host on which we are running.
*/
func GetOurIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Print(err)
		}
	}()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

/*
SearchForElectrolyser will turn on the relevant relay and search the subnet that we are in for an electorlyser to come on line.
If a new electrolyser is found it adds it to the chain.
*/
func SearchForElectrolyser() error {
	device := len(SystemStatus.Electrolysers)
	OurIP, err := GetOurIP()
	if err != nil {
		return err
	}

	// First we lock the electrolysers so they do not get turned off when we are searching
	SystemStatus.ElectrolyserLock = true
	defer func() { SystemStatus.ElectrolyserLock = false }()
	switch device {
	case 0:
		if err := mbusRTU.EL0OnOff(true); err != nil {
			log.Print(err)
		}
	case 1:
		if err := mbusRTU.EL1OnOff(true); err != nil {
			log.Print(err)
		}
	default:
		return fmt.Errorf("we already have two electrolysers registered")
	}

	// Delay for 15 seconds to let the electrolyser power up.
	time.Sleep(time.Second * 15)

	if IP := scan(OurIP); IP != nil {
		SystemStatus.Electrolysers = append(SystemStatus.Electrolysers, NewElectrolyser(IP))
	}
	return nil
}

/*
AcquireElectrolysers attempts to find two electrolysers
*/
func AcquireElectrolysers() {
	// Wait for the ModbusRTU system to get started so we can turn the relays on.
	for {
		if mbusRTU.Active {
			break
		}
		time.Sleep(time.Second * 5)
	}

	// Electrolyser to off if they are on.
	if mbusRTU.el0 || mbusRTU.el1 {
		if err := mbusRTU.EL0OnOff(false); err != nil {
			log.Println(err)
		}
		if err := mbusRTU.EL1OnOff(false); err != nil {
			log.Println(err)
		}

		time.Sleep(time.Second * 5)
	}

	// Clear any existing electrolyser registrations
	params.Electrolysers = nil
	// Make sure we turn the electrolysers off when we are done.
	defer func() {
		if err := mbusRTU.EL0OnOff(false); err != nil {
			log.Print(err)
		}
		if err := mbusRTU.EL1OnOff(false); err != nil {
			log.Print(err)
		}
	}()
	// Search for the first electrolyser
	if err := SearchForElectrolyser(); err == nil {
		// Give it 5 seconds then get the serial number
		time.Sleep(time.Second * 10)

		el := new(ElectrolyserConfig)
		log.Print("Searching for serial number")
		el.Serial = SystemStatus.Electrolysers[0].GetSerial()
		log.Print("Got serial, adding to settings.")
		el.IP = SystemStatus.Electrolysers[0].GetIPString()
		params.Electrolysers = append(params.Electrolysers, el)

		// Search for a second electrolyser
		if err := SearchForElectrolyser(); err == nil {
			time.Sleep(time.Second * 10)

			el := new(ElectrolyserConfig)
			el.Serial = SystemStatus.Electrolysers[1].GetSerial()
			el.IP = SystemStatus.Electrolysers[1].GetIPString()
			params.Electrolysers = append(params.Electrolysers, el)
		} else {
			log.Print(err)
		}
	} else {
		log.Print(err)
	}
	if len(params.Electrolysers) > 0 {
		if err := params.WriteSettings(); err != nil {
			log.Print(err)
		}
	}
	plural := ""
	if len(SystemStatus.Electrolysers) > 1 {
		plural = "s"
	}
	log.Printf("Found %d electrolyser%s", len(SystemStatus.Electrolysers), plural)
	if err := mbusRTU.EL0OnOff(false); err != nil {
		log.Print(err)
	}
	if err := mbusRTU.EL1OnOff(false); err != nil {
		log.Print(err)
	}
}

func tryConnect(host net.IP, port int) error {
	timeout := time.Millisecond * 250

	log.Println("Scanning", net.JoinHostPort(host.String(), fmt.Sprint(port)))
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host.String(), fmt.Sprint(port)), timeout)
	if err != nil {
		return err
	}
	if conn != nil {
		defer func() {
			if err := conn.Close(); err != nil {
				log.Print(err)
			}
		}()
		return nil
	}
	return fmt.Errorf("unknown error")
}

func CheckForElectrolyser(ip net.IP) error {
	var config modbus.ClientConfiguration
	config.Timeout = 1 * time.Second // 1 second timeout
	config.URL = "tcp://" + ip.String() + ":502"
	if Client, err := modbus.NewClient(&config); err == nil {
		if err := Client.Open(); err != nil {
			return err
		}
		defer func() {
			if err := Client.Close(); err != nil {
				log.Print(err)
			}
		}()
		model, err := Client.ReadUint32(0, modbus.INPUT_REGISTER)
		if err != nil {
			log.Println("Error getting serial number - ", err)
			return err
		}
		// Is this an EL21?
		if model != 0x454C3231 {
			return fmt.Errorf("not an EL21")
		}
		return nil
	} else {
		return err

	}
}

func ipRegistered(ip byte) bool {
	for _, el := range SystemStatus.Electrolysers {
		if el.ip[3] == ip {
			return true
		}
	}
	return false
}

func scan(OurIP net.IP) net.IP {
	IP := OurIP

	for ip := byte(254); ip > 1; ip-- {
		if (ip != OurIP[3]) && !ipRegistered(ip) {
			IP[3] = ip
			if tryConnect(IP, 502) == nil {
				if CheckForElectrolyser(IP) == nil {
					return IP
				}
			}
		}
	}
	return nil
}
