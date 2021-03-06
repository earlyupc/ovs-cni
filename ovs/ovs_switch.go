package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/John-Lin/ovsdb"
	"github.com/containernetworking/cni/pkg/types/current"
)

// OVSSwitch is a bridge instance
type OVSSwitch struct {
	NodeType     string
	BridgeName   string
	CtrlHostPort string
	ovsdb        *ovsdb.OvsDriver
}

// NewOVSSwitch for creating a ovs bridge
func NewOVSSwitch(bridgeName string) (*OVSSwitch, error) {
	sw := new(OVSSwitch)
	sw.NodeType = "OVSSwitch"
	sw.BridgeName = bridgeName

	sw.ovsdb = ovsdb.NewOvsDriverWithUnix(bridgeName)

	// Check if port is already part of the OVS and add it
	if !sw.ovsdb.IsPortNamePresent(bridgeName) {
		// Create an internal port in OVS
		err := sw.ovsdb.CreatePort(bridgeName, "internal", 0)
		if err != nil {
			return nil, err
		}
	}

	time.Sleep(300 * time.Millisecond)
	// log.Infof("Waiting for OVS bridge %s etup", bridgeName)

	// ip link set ovs up
	err := setLinkUp(bridgeName)
	if err != nil {
		return nil, err
	}

	return sw, nil
}

// addPort for asking OVSDB driver to add the port
func (sw *OVSSwitch) addPort(ifName string) error {
	if !sw.ovsdb.IsPortNamePresent(ifName) {
		err := sw.ovsdb.CreatePort(ifName, "", 0)
		if err != nil {
			return fmt.Errorf("Error creating the port, Err: %v", err)
		}
	}
	return nil
}

// delPort for asking OVSDB driver to delete the port
func (sw *OVSSwitch) delPort(ifName string) error {
	if sw.ovsdb.IsPortNamePresent(ifName) {
		err := sw.ovsdb.DeletePort(ifName)
		if err != nil {
			return fmt.Errorf("Error deleting the port, Err: %v", err)
		}
	}
	return nil
}

// SetCtrl for seting up OpenFlow controller for ovs bridge
func (sw *OVSSwitch) SetCtrl(hostport string) error {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return fmt.Errorf("Invalid controller IP and port. Err: %v", err)
	}
	uPort, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return fmt.Errorf("Invalid controller port number. Err: %v", err)
	}
	err = sw.ovsdb.AddController(host, uint16(uPort))
	if err != nil {
		return fmt.Errorf("Error adding controller to OVS. Err: %v", err)
	}
	sw.CtrlHostPort = hostport
	return nil
}

func (sw *OVSSwitch) Delete() error {
	if exist := sw.ovsdb.IsBridgePresent(sw.BridgeName); exist != true {
		return errors.New(sw.BridgeName + " doesn't exist, we can delete")
	}

	return sw.ovsdb.DeleteBridge(sw.BridgeName)
}

func (sw *OVSSwitch) AddVTEPs(VtepIPs []string) error {
	for _, v := range VtepIPs {
		intfName := vxlanIfName(v)
		isPresent, vsifName := sw.ovsdb.IsVtepPresent(v)

		if !isPresent || (vsifName != intfName) {
			//create VTEP
			err := sw.ovsdb.CreateVtep(intfName, v)
			if err != nil {
				return fmt.Errorf("Error creating VTEP port %s. Err: %v", intfName, err)
			}

		}
	}
	return nil
}

// OVSByName is a alias for finding a ovs by name and returns a pointer to the object.
func OVSByName(brName string) (*OVSSwitch, error) {
	return NewOVSSwitch(brName)
}

// createOVS is a helper function for create a ovs object
func createOVS(n *NetConf) (*OVSSwitch, *current.Interface, error) {
	// create bridge if necessary
	ovsbr, err := NewOVSSwitch(n.OVSBrName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup bridge %q: %v", n.OVSBrName, err)
	}

	return ovsbr, &current.Interface{
		Name: ovsbr.BridgeName,
	}, nil
}
