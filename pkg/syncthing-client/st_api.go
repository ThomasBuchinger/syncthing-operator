package syncthingclient

import (
	"errors"
)

type UseageReport int

const (
	AllowUsageReport UseageReport = 3
	DenyUsageReport  UseageReport = -1
)

type StConfig struct {
	Gui     GuiElement
	Options OptionsElement
	Devices []DeviceElement
	Folders []FolderElement
}

type GuiElement struct {
	Enabled, UseTls, InsecureAdminAccess, Debugging, SkipHostCheck, InsecureAllowFrameLoading bool
	Address, AuthMode, User, Password                                                         string
}

type OptionsElement struct {
	UrAccepted UseageReport
}
type DeviceElement struct {
	DeviceId, Name, Compression, CertName, IntroducedBy                        string
	Addresses, IgnoredFolders, AllowedNetworks                                 []string
	Introducer, SkipIntroductionRemovals, Paused, AutoAcceptFolders, Untrusted bool
	MaxSendKbps, MaxRecvKbps                                                   int
}
type FolderElement struct {
}

func (c StClient) GetDeviceIndexById(list []DeviceElement, id string) int {
	for i, device := range list {
		if device.DeviceId == id {
			return i
		}
	}
	return -1
}

func (c StClient) Ping() (bool, string) {
	pingReq, _ := c.newRequestTemplate("GET", "/rest/system/ping", nil)
	_, err := c.do(pingReq, nil)

	if err != nil {
		return false, err.Error()
	}

	return true, "Success"

}
func (c StClient) GetConfig() (StConfig, error) {
	config := StConfig{}
	request, _ := c.newRequestTemplate("GET", "/rest/config", nil)
	_, err := c.do(request, &config)
	return config, err
}

func (c StClient) SendUsageStatistics(sendIt UseageReport) error {
	value := struct {
		UrAccepted int
	}{
		UrAccepted: int(sendIt),
	}
	req, err := c.newRequestTemplate("PATCH", "/rest/config/options", value)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	}
	if response.StatusCode == 200 {
		return nil
	} else {
		return errors.New("Syncthing returned: " + response.Status)
	}
}
func (c StClient) SetAuth(username string, password string) error {
	newSettings := struct {
		AuthMode, User, Password string
	}{
		AuthMode: "static",
		User:     username,
		Password: password,
	}
	req, err := c.newRequestTemplate("PATCH", "/rest/config/gui", newSettings)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	}
	if response.StatusCode == 200 {
		return nil
	} else {
		return errors.New("Syncthing returned: " + response.Status)
	}
}

func (c StClient) ReplaceDevice(dev DeviceElement) error {
	req, err := c.newRequestTemplate("PUT", "/rest/config/devices/"+dev.DeviceId, dev)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	} else if response.StatusCode != 200 {
		return errors.New("PUT Request returned unexpected status: " + response.Status)
	}

	return nil
}
func (c StClient) DeleteDevice(deviceId string) error {
	req, _ := c.newRequestTemplate("DELETE", "/rest/config/devices/"+deviceId, nil)
	response, err := c.do(req, nil)
	if err != nil {
		return err
	} else if response.StatusCode != 200 {
		return errors.New("DELETE Request returned unexpected status: " + response.Status)
	}
	return nil
}
